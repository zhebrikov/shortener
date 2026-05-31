package handler

import (
	"net/http"
	"time"

	"github.com/zhebrikov/shortener/internal/audit"
	"github.com/zhebrikov/shortener/internal/auth"
)

func publishAudit(p *audit.Publisher, r *http.Request, action, rawURL string) {
	if p == nil {
		return
	}
	ev := audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		URL:    rawURL,
	}
	if uid, ok := auth.UserIDFromContext(r.Context()); ok {
		ev.UserID = uid
	}
	p.Publish(ev)
}
