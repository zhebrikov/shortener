package postgresql

import (
	"os"
	"testing"
)

func TestConnection_InvalidDSN_ReturnsError(t *testing.T) {
	_, err := Connection("postgres://invalid-host:9999/nonexistent?sslmode=disable")
	if err == nil {
		t.Error("Connection: ожидалась ошибка при невалидном DSN, получен nil")
	}
}

func TestConnection_EmptyDSN_ReturnsError(t *testing.T) {
	_, err := Connection("")
	if err == nil {
		t.Error("Connection: ожидалась ошибка при пустом DSN, получен nil")
	}
}

func TestConnection_ValidDSN_Integration(t *testing.T) {
	dsn := os.Getenv("SHORTENER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SHORTENER_TEST_POSTGRES_DSN не задан, интеграционный тест пропущен")
	}

	db, err := Connection(dsn)
	if err != nil {
		t.Fatalf("Connection: неожиданная ошибка: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Errorf("Connection: после успешного подключения Ping вернул ошибку: %v", err)
	}
}
