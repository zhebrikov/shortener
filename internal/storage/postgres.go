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
	rows, err := p.db.Query("SELECT url, short_url FROM links ORDER BY id")
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
		links = append(links, Link{UUID: i, ShortURL: shortURL, OriginalURL: url})
	}
	return links, rows.Err()
}

// GetByShortURL возвращает одну запись по short_url или sql.ErrNoRows, если не найдена.
func (p *PostgresStorage) GetByShortURL(shortURL string) (*Link, error) {
	var url, short string
	err := p.db.QueryRow("SELECT url, short_url FROM links WHERE short_url = $1", shortURL).Scan(&url, &short)
	if err != nil {
		return nil, err
	}
	return &Link{ShortURL: short, OriginalURL: url}, nil
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
	_, err := p.db.Exec(
		"INSERT INTO links (url, short_url) VALUES ($1, $2)",
		link.OriginalURL,
		link.ShortURL,
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
	valueArgs := make([]interface{}, 0, len(links)*2)
	for i, link := range links {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		valueArgs = append(valueArgs, link.OriginalURL, link.ShortURL)
	}
	stmt := "INSERT INTO links (url, short_url) VALUES " + strings.Join(valueStrings, ",")
	_, err := p.db.Exec(stmt, valueArgs...)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == pgerrcode.UniqueViolation {
			return ErrDuplicateURL
		}
		return err
	}
	return nil
}
