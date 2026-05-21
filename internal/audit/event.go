// Package audit реализует паттерн «Наблюдатель» для записи событий сокращения и переходов.
package audit

const (
	// ActionShorten — пользователь сократил URL.
	ActionShorten = "shorten"
	// ActionFollow — пользователь перешёл по короткой ссылке.
	ActionFollow = "follow"
)

// Event описывает одну запись аудита (JSON в файл или тело POST).
type Event struct {
	Ts     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}
