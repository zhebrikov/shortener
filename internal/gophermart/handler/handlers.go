// Package handler implements the Gophermart HTTP API.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zhebrikov/shortener/internal/gophermart/luhn"
	"github.com/zhebrikov/shortener/internal/gophermart/store"
	"golang.org/x/crypto/bcrypt"
)

// API wires HTTP handlers to a Storer and JWT secret.
type API struct {
	Store  store.Storer
	Secret string
	// HashPassword produces a bcrypt hash for a new user's password. When nil
	// [bcrypt.GenerateFromPassword] is used; tests override it to exercise
	// hashing failure paths.
	HashPassword func(password []byte, cost int) ([]byte, error)
}

func (a *API) hashPassword(password []byte, cost int) ([]byte, error) {
	if a.HashPassword != nil {
		return a.HashPassword(password, cost)
	}
	return bcrypt.GenerateFromPassword(password, cost)
}

type credsBody struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Register handles POST /api/user/register.
func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body credsBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Login = strings.TrimSpace(body.Login)
	body.Password = strings.TrimSpace(body.Password)
	if body.Login == "" || body.Password == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	hash, err := a.hashPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	id, err := a.Store.RegisterUser(r.Context(), body.Login, string(hash))
	if errors.Is(err, store.ErrLoginTaken) {
		http.Error(w, "login taken", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	SetSessionCookie(w, a.Secret, id)
	w.WriteHeader(http.StatusOK)
}

// Login handles POST /api/user/login.
func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body credsBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Login = strings.TrimSpace(body.Login)
	body.Password = strings.TrimSpace(body.Password)
	if body.Login == "" || body.Password == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	id, hash, err := a.Store.GetUserByLogin(r.Context(), body.Login)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	SetSessionCookie(w, a.Secret, id)
	w.WriteHeader(http.StatusOK)
}

// UploadOrder handles POST /api/user/orders.
func (a *API) UploadOrder(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	number := strings.TrimSpace(string(raw))
	if number == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !luhn.Valid(number) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}
	res, err := a.Store.UploadOrder(r.Context(), uid, number)
	if errors.Is(err, store.ErrOrderOwnedByOtherUser) {
		http.Error(w, "conflict", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	switch res {
	case store.UploadAlreadySameUser:
		w.WriteHeader(http.StatusOK)
	case store.UploadAccepted:
		w.WriteHeader(http.StatusAccepted)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

type orderResp struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

// ListOrders handles GET /api/user/orders.
func (a *API) ListOrders(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out := make([]orderResp, 0)
	for row, err := range a.Store.ListUserOrders(r.Context(), uid) {
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		out = append(out, orderResp{
			Number:     row.Number,
			Status:     row.Status,
			Accrual:    row.Accrual,
			UploadedAt: row.UploadedAt.Format(time.RFC3339),
		})
	}
	if len(out) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type balanceResp struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// Balance handles GET /api/user/balance.
func (a *API) Balance(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cur, wdrawn, err := a.Store.Balance(r.Context(), uid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(balanceResp{Current: cur, Withdrawn: wdrawn})
}

type withdrawBody struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// Withdraw handles POST /api/user/balance/withdraw.
func (a *API) Withdraw(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body withdrawBody
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	body.Order = strings.TrimSpace(body.Order)
	if body.Order == "" || body.Sum <= 0 {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !luhn.Valid(body.Order) {
		http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		return
	}
	err := a.Store.Withdraw(r.Context(), uid, body.Order, body.Sum)
	if errors.Is(err, store.ErrInsufficientFunds) {
		http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type withdrawalResp struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

// ListWithdrawals handles GET /api/user/withdrawals.
func (a *API) ListWithdrawals(w http.ResponseWriter, r *http.Request) {
	uid, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := a.Store.ListWithdrawals(r.Context(), uid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	out := make([]withdrawalResp, 0, len(rows))
	for _, row := range rows {
		out = append(out, withdrawalResp{
			Order:       row.OrderNumber,
			Sum:         row.Sum,
			ProcessedAt: row.ProcessedAt.Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
