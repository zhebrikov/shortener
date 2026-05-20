package audit

const (
	ActionShorten = "shorten"
	ActionFollow  = "follow"
)

// Event описывает одну запись аудита (JSON в файл или тело POST).
type Event struct {
	Ts     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}
