package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// HTTPObserver отправляет событие POST-ом на auditURL.
type HTTPObserver struct {
	auditURL string
	client   *http.Client
}

// NewHTTPObserver создаёт наблюдателя для удалённого приёмника.
func NewHTTPObserver(auditURL string, client *http.Client) *HTTPObserver {
	return &HTTPObserver{auditURL: auditURL, client: client}
}

// OnAudit реализует Observer.
func (h *HTTPObserver) OnAudit(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.auditURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
