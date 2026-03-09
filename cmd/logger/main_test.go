package main

import (
	"regexp"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	got := run()

	// Префикс логгера
	if !strings.Contains(got, "mylog: ") {
		t.Errorf("run() output must contain prefix %q; got:\n%s", "mylog: ", got)
	}

	// Сообщения
	if !strings.Contains(got, "Hello, world!") {
		t.Errorf("run() output must contain %q; got:\n%s", "Hello, world!", got)
	}
	if !strings.Contains(got, "Goodbye") {
		t.Errorf("run() output must contain %q; got:\n%s", "Goodbye", got)
	}

	// Формат LstdFlags: префикс "mylog: ", затем "2006/01/02 15:04:05 "
	timeFormat := regexp.MustCompile(`mylog: \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} `)
	if !timeFormat.MatchString(got) {
		t.Errorf("run() output must match LstdFlags (mylog: YYYY/MM/DD HH:MM:SS ); got:\n%s", got)
	}

	// Две строки лога
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Errorf("run() must produce 2 log lines; got %d:\n%s", len(lines), got)
	}
}
