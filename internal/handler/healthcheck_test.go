package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHealthCheck(t *testing.T) {
	t.Run("БД доступна — 200 OK", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer db.Close()

		mock.ExpectPing().WillReturnError(nil)

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rr := httptest.NewRecorder()

		HealthCheck(db)(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("HealthCheck: статус = %d, ожидалось %d", rr.Code, http.StatusOK)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("ожидания sqlmock не выполнены: %v", err)
		}
	})

	t.Run("БД не настроена (nil) — 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rr := httptest.NewRecorder()
		HealthCheck(nil)(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("HealthCheck(nil): статус = %d, ожидалось %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("БД недоступна — 500 Internal Server Error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("sqlmock.New: %v", err)
		}
		defer db.Close()

		mock.ExpectPing().WillReturnError(errors.New("connection refused"))

		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		rr := httptest.NewRecorder()

		HealthCheck(db)(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("HealthCheck: статус = %d, ожидалось %d", rr.Code, http.StatusInternalServerError)
		}
		if body := rr.Body.String(); body != "Internal Server Error\n" {
			t.Errorf("HealthCheck: тело ответа = %q, ожидалось %q", body, "Internal Server Error\n")
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("ожидания sqlmock не выполнены: %v", err)
		}
	})
}
