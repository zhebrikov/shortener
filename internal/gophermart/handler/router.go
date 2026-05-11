package handler

import (
	"net/http"

	"github.com/zhebrikov/shortener/internal/middleware"
)

// NewRouter returns the HTTP mux for the Gophermart service.
func NewRouter(api *API) http.Handler {
	mux := http.NewServeMux()
	auth := AuthMiddleware(api.Secret)
	withAuth := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			auth(h).ServeHTTP(w, r)
		}
	}

	mux.HandleFunc("POST /api/user/register", api.Register)
	mux.HandleFunc("POST /api/user/login", api.Login)
	mux.HandleFunc("POST /api/user/orders", withAuth(http.HandlerFunc(api.UploadOrder)))
	mux.HandleFunc("GET /api/user/orders", withAuth(http.HandlerFunc(api.ListOrders)))
	mux.HandleFunc("GET /api/user/balance", withAuth(http.HandlerFunc(api.Balance)))
	mux.HandleFunc("POST /api/user/balance/withdraw", withAuth(http.HandlerFunc(api.Withdraw)))
	mux.HandleFunc("GET /api/user/withdrawals", withAuth(http.HandlerFunc(api.ListWithdrawals)))

	return middleware.Gzip(mux)
}
