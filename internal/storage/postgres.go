package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

// PostgresStorage — хранилище ссылок в PostgreSQL.
type PostgresStorage struct {
	db *sql.DB
}

// Проверка, что *PostgresStorage реализует LinkStore.
var _ LinkStore = (*PostgresStorage)(nil)

// NewPostgresStorage создаёт хранилище, использующее таблицу links (url, short_url).
func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

// ReadStorage возвращает все строки таблицы links.
func (p *PostgresStorage) ReadStorage() ([]Link, error) {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, "SELECT url, short_url, user_id, is_deleted FROM links ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for i := 1; rows.Next(); i++ {
		var url, shortURL string
		var userID sql.NullString
		var isDeleted bool
		if err := rows.Scan(&url, &shortURL, &userID, &isDeleted); err != nil {
			return nil, err
		}
		uid := ""
		if userID.Valid {
			uid = userID.String
		}
		links = append(links, Link{UUID: i, ShortURL: shortURL, OriginalURL: url, UserID: uid, IsDeleted: isDeleted})
	}
	return links, rows.Err()
}

// GetByShortURL возвращает одну запись по short_url или sql.ErrNoRows, если не найдена.
func (p *PostgresStorage) GetByShortURL(shortURL string) (*Link, error) {
	ctx := context.Background()
	var url, short string
	var userID sql.NullString
	var isDeleted bool
	err := p.db.QueryRowContext(ctx, "SELECT url, short_url, user_id, is_deleted FROM links WHERE short_url = $1", shortURL).Scan(&url, &short, &userID, &isDeleted)
	if err != nil {
		return nil, err
	}
	uid := ""
	if userID.Valid {
		uid = userID.String
	}
	return &Link{ShortURL: short, OriginalURL: url, UserID: uid, IsDeleted: isDeleted}, nil
}

// GetShortURLByOriginalURL возвращает short_url по полю url.
func (p *PostgresStorage) GetShortURLByOriginalURL(originalURL string) (string, error) {
	ctx := context.Background()
	var shortURL string
	err := p.db.QueryRowContext(ctx, "SELECT short_url FROM links WHERE url = $1", originalURL).Scan(&shortURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("url not found")
		}
		return "", err
	}
	return shortURL, nil
}

// WriteStorage вставляет одну строку; уникальное нарушение по url даёт ErrDuplicateURL.
func (p *PostgresStorage) WriteStorage(link Link) error {
	ctx := context.Background()
	var userID interface{}
	if link.UserID != "" {
		userID = link.UserID
	}
	_, err := p.db.ExecContext(
		ctx,
		"INSERT INTO links (url, short_url, user_id, is_deleted) VALUES ($1, $2, $3, $4)",
		link.OriginalURL,
		link.ShortURL,
		userID,
		false,
	)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pgerrcode.UniqueViolation {
			return ErrDuplicateURL
		}
		return err
	}
	return nil
}

// WriteStorageBatch вставляет все ссылки одним запросом (один roundtrip к БД).
func (p *PostgresStorage) WriteStorageBatch(links []Link) error {
	if len(links) == 0 {
		return nil
	}
	ctx := context.Background()
	valueStrings := make([]string, 0, len(links))
	valueArgs := make([]interface{}, 0, len(links)*4)
	for i, link := range links {
		n := i * 4
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4))
		var uid interface{}
		if link.UserID != "" {
			uid = link.UserID
		}
		valueArgs = append(valueArgs, link.OriginalURL, link.ShortURL, uid, false)
	}
	stmt := "INSERT INTO links (url, short_url, user_id, is_deleted) VALUES " + strings.Join(valueStrings, ",")
	_, err := p.db.ExecContext(ctx, stmt, valueArgs...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pgerrcode.UniqueViolation {
			return ErrDuplicateURL
		}
		return err
	}
	return nil
}

// GetLinksByUserID возвращает неудалённые ссылки пользователя.
func (p *PostgresStorage) GetLinksByUserID(userID string) ([]Link, error) {
	if userID == "" {
		return nil, nil
	}
	ctx := context.Background()
	rows, err := p.db.QueryContext(
		ctx,
		"SELECT url, short_url FROM links WHERE user_id = $1::uuid AND is_deleted = false ORDER BY id",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for i := 1; rows.Next(); i++ {
		var url, shortURL string
		if err := rows.Scan(&url, &shortURL); err != nil {
			return nil, err
		}
		links = append(links, Link{UUID: i, ShortURL: shortURL, OriginalURL: url, UserID: userID})
	}
	return links, rows.Err()
}

// SoftDeleteURLsByUser выставляет is_deleted для строк, где short_url совпадает с идентификатором или оканчивается на /code.
func (p *PostgresStorage) SoftDeleteURLsByUser(userID string, shortCodes []string) error {
	if userID == "" || len(shortCodes) == 0 {
		return nil
	}
	ctx := context.Background()
	_, err := p.db.ExecContext(ctx, `
		UPDATE links SET is_deleted = true
		WHERE user_id = $1::uuid
		AND is_deleted = false
		AND (
			short_url = ANY($2::text[])
			OR EXISTS (
				SELECT 1 FROM unnest($2::text[]) AS x(code)
				WHERE links.short_url LIKE '%/' || x.code
			)
		)
	`, userID, pq.Array(shortCodes))
	return err
}

// GetLinkByShortCode ищет строку по коду (полный short_url или суффикс пути).
func (p *PostgresStorage) GetLinkByShortCode(shortCode string) (Link, error) {
	if shortCode == "" {
		return Link{}, ErrLinkNotFound
	}
	ctx := context.Background()
	var url, short string
	var userID sql.NullString
	var isDeleted bool
	err := p.db.QueryRowContext(
		ctx,
		`SELECT url, short_url, user_id, is_deleted FROM links
		 WHERE short_url = $1 OR short_url LIKE '%/' || $1`,
		shortCode,
	).Scan(&url, &short, &userID, &isDeleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Link{}, ErrLinkNotFound
		}
		return Link{}, err
	}
	uid := ""
	if userID.Valid {
		uid = userID.String
	}
	return Link{ShortURL: short, OriginalURL: url, UserID: uid, IsDeleted: isDeleted}, nil
}

// CountURLs возвращает общее количество ссылок в таблице links.
func (p *PostgresStorage) CountURLs() (int, error) {
	ctx := context.Background()
	var count int
	err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&count)
	return count, err
}

// CountUsers возвращает количество уникальных пользователей.
func (p *PostgresStorage) CountUsers() (int, error) {
	ctx := context.Background()
	var count int
	err := p.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT user_id) FROM links WHERE user_id IS NOT NULL").Scan(&count)
	return count, err
}

// Stats возвращает количество URL и пользователей в одной read-only транзакции.
func (p *PostgresStorage) Stats() (int, int, error) {
	ctx := context.Background()
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	var urls, users int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&urls); err != nil {
		return 0, 0, err
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(DISTINCT user_id) FROM links WHERE user_id IS NOT NULL").Scan(&users); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return urls, users, nil
}
