// Package accrual provides an HTTP client for the external loyalty accrual system.
package accrual

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// OrderStatus mirrors the accrual API processing states.
type OrderStatus string

const (
	StatusRegistered OrderStatus = "REGISTERED"
	StatusInvalid    OrderStatus = "INVALID"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusProcessed  OrderStatus = "PROCESSED"
)

// OrderInfo is the JSON body returned by GET /api/orders/{number} on success.
type OrderInfo struct {
	Order   string      `json:"order"`
	Status  OrderStatus `json:"status"`
	Accrual *float64    `json:"accrual,omitempty"`
}

// ErrNotRegistered means the accrual system returned HTTP 204 for the order number.
var ErrNotRegistered = errors.New("order not registered in accrual system")

// ErrTooManyRequests is returned when the accrual system responds with HTTP 429.
type ErrTooManyRequests struct {
	// RetryAfter is the duration suggested by Retry-After header, if present.
	RetryAfter time.Duration
	Body       string
}

func (e *ErrTooManyRequests) Error() string {
	return fmt.Sprintf("accrual rate limited: retry after %v", e.RetryAfter)
}

// Client calls the trusted accrual HTTP API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient constructs a Client that resolves order URLs under baseURL (no trailing slash).
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

// GetOrder requests accrual information for a single order number.
func (c *Client) GetOrder(orderNumber string) (*OrderInfo, error) {
	url := fmt.Sprintf("%s/api/orders/%s", c.baseURL, orderNumber)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	switch resp.StatusCode {
	case http.StatusOK:
		var info OrderInfo
		if err := json.Unmarshal(body, &info); err != nil {
			return nil, fmt.Errorf("decode accrual response: %w", err)
		}
		return &info, nil
	case http.StatusNoContent:
		return nil, ErrNotRegistered
	case http.StatusTooManyRequests:
		ra := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, &ErrTooManyRequests{RetryAfter: ra, Body: string(body)}
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("accrual server error: %s", strings.TrimSpace(string(body)))
	default:
		return nil, fmt.Errorf("accrual unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return time.Minute
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return time.Minute
}
