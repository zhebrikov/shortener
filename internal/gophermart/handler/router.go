package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/zhebrikov/shortener/internal/middleware"
)

// NewRouter returns the full chi router for the Gophermart service.
func NewRouter(api *API) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Gzip)

	r.Post("/api/user/register", api.Register)
	r.Post("/api/user/login", api.Login)

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(api.Secret))
		r.Post("/api/user/orders", api.UploadOrder)
		r.Get("/api/user/orders", api.ListOrders)
		r.Get("/api/user/balance", api.Balance)
		r.Post("/api/user/balance/withdraw", api.Withdraw)
		r.Get("/api/user/withdrawals", api.ListWithdrawals)
	})

	return r
}
