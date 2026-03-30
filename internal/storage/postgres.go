package storage

import (
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

func (p *PostgresStorage) ReadStorage() ([]Link, error) {
	rows, err := p.db.Query("SELECT url, short_url, user_id FROM links ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []Link
	for i := 1; rows.Next(); i++ {
		var url, shortURL string
		var userID sql.NullString
		if err := rows.Scan(&url, &shortURL, &userID); err != nil {
			return nil, err
		}
		uid := ""
		if userID.Valid {
			uid = userID.String
		}
		links = append(links, Link{UUID: i, ShortURL: shortURL, OriginalURL: url, UserID: uid})
	}
	return links, rows.Err()
}

// GetByShortURL возвращает одну запись по short_url или sql.ErrNoRows, если не найдена.
func (p *PostgresStorage) GetByShortURL(shortURL string) (*Link, error) {
	var url, short string
	var userID sql.NullString
	err := p.db.QueryRow("SELECT url, short_url, user_id FROM links WHERE short_url = $1", shortURL).Scan(&url, &short, &userID)
	if err != nil {
		return nil, err
	}
	uid := ""
	if userID.Valid {
		uid = userID.String
	}
	return &Link{ShortURL: short, OriginalURL: url, UserID: uid}, nil
}

func (p *PostgresStorage) GetShortURLByOriginalURL(originalURL string) (string, error) {
	var shortURL string
	err := p.db.QueryRow("SELECT short_url FROM links WHERE url = $1", originalURL).Scan(&shortURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("url not found")
		}
		return "", err
	}
	return shortURL, nil
}

func (p *PostgresStorage) WriteStorage(link Link) error {
	var userID interface{}
	if link.UserID != "" {
		userID = link.UserID
	}
	_, err := p.db.Exec(
		"INSERT INTO links (url, short_url, user_id) VALUES ($1, $2, $3)",
		link.OriginalURL,
		link.ShortURL,
		userID,
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
	valueStrings := make([]string, 0, len(links))
	valueArgs := make([]interface{}, 0, len(links)*3)
	for i, link := range links {
		n := i * 3
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", n+1, n+2, n+3))
		var uid interface{}
		if link.UserID != "" {
			uid = link.UserID
		}
		valueArgs = append(valueArgs, link.OriginalURL, link.ShortURL, uid)
	}
	stmt := "INSERT INTO links (url, short_url, user_id) VALUES " + strings.Join(valueStrings, ",")
	_, err := p.db.Exec(stmt, valueArgs...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pgerrcode.UniqueViolation {
			return ErrDuplicateURL
		}
		return err
	}
	return nil
}

func (p *PostgresStorage) GetLinksByUserID(userID string) ([]Link, error) {
	if userID == "" {
		return nil, nil
	}
	rows, err := p.db.Query(
		"SELECT url, short_url FROM links WHERE user_id = $1::uuid ORDER BY id",
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
