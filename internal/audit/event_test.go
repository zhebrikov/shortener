package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEvent_JSON_omitemptyUserID(t *testing.T) {
	ev := Event{Ts: 1, Action: ActionShorten, URL: "https://u"}
	b, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "user_id") {
		t.Errorf("expected no user_id, got %s", b)
	}
	ev2 := Event{Ts: 1, Action: ActionFollow, UserID: "id1", URL: "https://u"}
	b2, err := json.Marshal(ev2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b2), `"user_id":"id1"`) {
		t.Errorf("want user_id in JSON, got %s", b2)
	}
}
