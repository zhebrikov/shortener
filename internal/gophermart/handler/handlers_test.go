package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhebrikov/shortener/internal/gophermart/store"
	"golang.org/x/crypto/bcrypt"
)

// panicStore implements store.Storer with panicking defaults for methods not overridden in a test.
type panicStore struct{}

func (panicStore) RegisterUser(context.Context, string, string) (string, error) {
	panic("RegisterUser")
}
func (panicStore) GetUserByLogin(context.Context, string) (string, string, error) {
	panic("GetUserByLogin")
}
func (panicStore) UploadOrder(context.Context, string, string) (store.OrderUploadResult, error) {
	panic("UploadOrder")
}
func (panicStore) ListUserOrders(context.Context, string) iter.Seq2[store.OrderRow, error] {
	return func(yield func(store.OrderRow, error) bool) { panic("ListUserOrders") }
}
func (panicStore) Balance(context.Context, string) (float64, float64, error) { panic("Balance") }
func (panicStore) Withdraw(context.Context, string, string, float64) error   { panic("Withdraw") }
func (panicStore) ListWithdrawals(context.Context, string) ([]store.WithdrawalRow, error) {
	panic("ListWithdrawals")
}
func (panicStore) PendingAccrualJobs(context.Context, int) ([]store.AccrualJob, error) {
	panic("PendingAccrualJobs")
}
func (panicStore) SyncAccrualStatus(context.Context, int64, string, *float64) error {
	panic("SyncAccrualStatus")
}

type regMock struct {
	panicStore
	id  string
	err error
}

func (m regMock) RegisterUser(ctx context.Context, login, passwordHash string) (string, error) {
	_ = ctx
	_ = login
	_ = passwordHash
	return m.id, m.err
}

func TestAPIRegisterOK(t *testing.T) {
	api := &API{
		Store:  regMock{id: "user-uuid-1"},
		Secret: "test-secret-key-32bytes-long!!",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"login":"alice","password":"secret"}`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != CookieName || cookies[0].Value == "" {
		t.Fatalf("cookies %+v", cookies)
	}
}

func TestAPIRegisterConflict(t *testing.T) {
	api := &API{
		Store:  regMock{err: store.ErrLoginTaken},
		Secret: "test-secret-key-32bytes-long!!",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"login":"a","password":"b"}`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

type loginMock struct {
	panicStore
	hash []byte
}

func (m loginMock) GetUserByLogin(ctx context.Context, login string) (string, string, error) {
	_ = ctx
	if login != "alice" {
		return "", "", sql.ErrNoRows
	}
	return "uid-1", string(m.hash), nil
}

func TestAPILoginOK(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("mypass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	api := &API{Store: loginMock{hash: hash}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(`{"login":"alice","password":"mypass"}`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAPILoginUnauthorized(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("other"), bcrypt.MinCost)
	api := &API{Store: loginMock{hash: hash}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(`{"login":"alice","password":"wrong"}`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

type uploadMock struct {
	panicStore
	res store.OrderUploadResult
	err error
}

func (m uploadMock) UploadOrder(ctx context.Context, userID, number string) (store.OrderUploadResult, error) {
	_ = ctx
	_ = userID
	_ = number
	return m.res, m.err
}

func TestAPIUploadOrderUnauthenticated(t *testing.T) {
	api := &API{Store: uploadMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("12345678903"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAPIUploadOrderLuhnInvalid(t *testing.T) {
	api := &API{Store: uploadMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", bytes.NewBufferString("123"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", rec.Code)
	}
}

type withdrawMock struct {
	panicStore
	err error
}

func (m withdrawMock) Withdraw(ctx context.Context, userID, orderNumber string, sum float64) error {
	_ = ctx
	_ = userID
	_ = orderNumber
	_ = sum
	return m.err
}

func TestAPIWithdraw402(t *testing.T) {
	api := &API{
		Store:  withdrawMock{err: store.ErrInsufficientFunds},
		Secret: "secret-key-32bytes-minimum-length!",
	}
	b, _ := json.Marshal(map[string]any{"order": "12345678903", "sum": 999999})
	req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(b))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status %d", rec.Code)
	}
}
