package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zhebrikov/shortener/internal/auth"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
)

type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errors.New("read") }

func TestAPIRegister_methodNotAllowed(t *testing.T) {
	api := &API{Store: regMock{id: "x"}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIRegister_badJSON(t *testing.T) {
	api := &API{Store: regMock{id: "x"}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIRegister_emptyFields(t *testing.T) {
	api := &API{Store: regMock{id: "x"}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"  ","password":""}`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIRegister_bcryptError(t *testing.T) {
	old := generatePasswordHash
	generatePasswordHash = func([]byte, int) ([]byte, error) { return nil, errors.New("bcrypt") }
	defer func() { generatePasswordHash = old }()

	api := &API{Store: regMock{id: "x"}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"a","password":"b"}`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIRegister_storeError(t *testing.T) {
	api := &API{Store: regMock{err: errors.New("db")}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"a","password":"b"}`))
	rec := httptest.NewRecorder()
	api.Register(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPILogin_methodNotAllowed(t *testing.T) {
	api := &API{Store: loginMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPILogin_badJSON(t *testing.T) {
	api := &API{Store: loginMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPILogin_emptyFields(t *testing.T) {
	api := &API{Store: loginMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"","password":"x"}`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPILogin_noSuchUser(t *testing.T) {
	api := &API{Store: loginMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"nobody","password":"x"}`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

type uploadResultMock struct {
	panicStore
	res store.OrderUploadResult
	err error
}

func (m uploadResultMock) UploadOrder(ctx context.Context, userID, number string) (store.OrderUploadResult, error) {
	return m.res, m.err
}

func TestAPIUploadOrder_readError(t *testing.T) {
	api := &API{Store: uploadMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", errReader{})
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_emptyBody(t *testing.T) {
	api := &API{Store: uploadMock{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("  \n"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_conflict(t *testing.T) {
	api := &API{Store: uploadMock{err: store.ErrOrderOwnedByOtherUser}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345678903"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_internal(t *testing.T) {
	api := &API{Store: uploadMock{err: errors.New("db")}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345678903"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_accepted(t *testing.T) {
	api := &API{Store: uploadMock{res: store.UploadAccepted}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345678903"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_sameUser(t *testing.T) {
	api := &API{Store: uploadMock{res: store.UploadAlreadySameUser}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345678903"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIUploadOrder_unknownResult(t *testing.T) {
	api := &API{Store: uploadResultMock{res: store.OrderUploadResult(99)}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("12345678903"))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.UploadOrder(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

type listOrdersMock struct {
	panicStore
	rows []store.OrderRow
	err  error
}

func (m listOrdersMock) ListUserOrders(ctx context.Context, uid string) ([]store.OrderRow, error) {
	return m.rows, m.err
}

func TestAPIListOrders_unauthorized(t *testing.T) {
	api := &API{Store: listOrdersMock{}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	api.ListOrders(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListOrders_error(t *testing.T) {
	api := &API{Store: listOrdersMock{err: errors.New("db")}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListOrders(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListOrders_noContent(t *testing.T) {
	api := &API{Store: listOrdersMock{rows: nil}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListOrders(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListOrders_json(t *testing.T) {
	ts := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	v := 1.0
	api := &API{Store: listOrdersMock{rows: []store.OrderRow{{
		Number: "1", Status: "PROCESSED", Accrual: &v, UploadedAt: ts,
	}}}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListOrders(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

type balMock struct {
	panicStore
	cur, w float64
	err    error
}

func (m balMock) Balance(ctx context.Context, uid string) (float64, float64, error) {
	return m.cur, m.w, m.err
}

func TestAPIBalance_unauthorized(t *testing.T) {
	api := &API{Store: balMock{}, Secret: "s"}
	rec := httptest.NewRecorder()
	api.Balance(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIBalance_error(t *testing.T) {
	api := &API{Store: balMock{err: errors.New("db")}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Balance(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIBalance_ok(t *testing.T) {
	api := &API{Store: balMock{cur: 10, w: 2}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Balance(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_unauthorized(t *testing.T) {
	api := &API{Store: withdrawMock{}, Secret: "s"}
	rec := httptest.NewRecorder()
	api.Withdraw(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_badJSON(t *testing.T) {
	api := &API{Store: withdrawMock{}, Secret: "s"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{`))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_invalidFields(t *testing.T) {
	api := &API{Store: withdrawMock{}, Secret: "s"}
	b, _ := json.Marshal(map[string]any{"order": "", "sum": 0})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_luhnInvalid(t *testing.T) {
	api := &API{Store: withdrawMock{}, Secret: "s"}
	b, _ := json.Marshal(map[string]any{"order": "123", "sum": 10})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_internal(t *testing.T) {
	api := &API{Store: withdrawMock{err: errors.New("db")}, Secret: "s"}
	b, _ := json.Marshal(map[string]any{"order": "12345678903", "sum": 1})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIWithdraw_ok(t *testing.T) {
	api := &API{Store: withdrawMock{}, Secret: "s"}
	b, _ := json.Marshal(map[string]any{"order": "12345678903", "sum": 1})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.Withdraw(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

type listWDMock struct {
	panicStore
	rows []store.WithdrawalRow
	err  error
}

func (m listWDMock) ListWithdrawals(ctx context.Context, uid string) ([]store.WithdrawalRow, error) {
	return m.rows, m.err
}

func TestAPIListWithdrawals_unauthorized(t *testing.T) {
	api := &API{Store: listWDMock{}, Secret: "s"}
	rec := httptest.NewRecorder()
	api.ListWithdrawals(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListWithdrawals_error(t *testing.T) {
	api := &API{Store: listWDMock{err: errors.New("db")}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListWithdrawals(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListWithdrawals_noContent(t *testing.T) {
	api := &API{Store: listWDMock{}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListWithdrawals(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAPIListWithdrawals_ok(t *testing.T) {
	ts := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	api := &API{Store: listWDMock{rows: []store.WithdrawalRow{{
		OrderNumber: "12345678903", Sum: 3, ProcessedAt: ts,
	}}}, Secret: "s"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(WithUserID(req.Context(), "u1"))
	rec := httptest.NewRecorder()
	api.ListWithdrawals(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

func TestNewRouter_smoke(t *testing.T) {
	secret := "secret-key-32bytes-minimum-length!"
	token := auth.SignUserID("user-1", secret)
	r := NewRouter(&API{Store: listOrdersMock{rows: []store.OrderRow{}}, Secret: secret})

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAuthMiddleware_cookieOK(t *testing.T) {
	secret := "secret-key-32bytes-minimum-length!"
	h := AuthMiddleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserIDFromContext(r.Context()); !ok {
			http.Error(w, "no ctx", http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusTeapot)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: auth.SignUserID("u1", secret)})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAuthMiddleware_bearerOK(t *testing.T) {
	secret := "secret-key-32bytes-minimum-length!"
	tok := auth.SignUserID("u2", secret)
	h := AuthMiddleware(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := UserIDFromContext(r.Context())
		if uid != "u2" {
			http.Error(w, "bad", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "  Bearer  "+tok+"  ")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAuthMiddleware_noToken(t *testing.T) {
	h := AuthMiddleware("s")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next called")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAuthMiddleware_badToken(t *testing.T) {
	h := AuthMiddleware("secret-key-32bytes-minimum-length!")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "not-a-jwt"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}

func TestUserIDFromContext_emptySubject(t *testing.T) {
	ctx := WithUserID(context.Background(), "")
	if _, ok := UserIDFromContext(ctx); ok {
		t.Fatal("expected false")
	}
}

func TestTokenFromRequest_shortBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bear") // len <= 7
	if got := tokenFromRequest(req); got != "" {
		t.Fatalf("%q", got)
	}
}

func TestTokenFromRequest_cookieMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := tokenFromRequest(req); got != "" {
		t.Fatalf("%q", got)
	}
}

func TestTokenFromRequest_cookieEmptyValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: CookieName, Value: "  "})
	if got := tokenFromRequest(req); got != "" {
		t.Fatalf("%q", got)
	}
}

type dbErrLogin struct{ panicStore }

func (dbErrLogin) GetUserByLogin(context.Context, string) (string, string, error) {
	return "", "", errors.New("db")
}

func TestAPILogin_GetUserDBError(t *testing.T) {
	api := &API{Store: dbErrLogin{}, Secret: "secret-key-32bytes-minimum-length!"}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"login":"alice","password":"x"}`))
	rec := httptest.NewRecorder()
	api.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d", rec.Code)
	}
}
