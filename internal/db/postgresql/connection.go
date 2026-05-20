// Package postgresql предоставляет подключение к PostgreSQL через database/sql.
package postgresql

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5"
)

// Connection открывает пул соединений по DSN и проверяет доступность Ping.
func Connection(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
