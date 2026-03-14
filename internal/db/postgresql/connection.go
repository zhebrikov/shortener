package postgresql

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5"
)

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
