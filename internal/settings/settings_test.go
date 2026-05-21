package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettings(t *testing.T) {
	fname := filepath.Join(t.TempDir(), "settings.json")
	settings := Settings{
		Port: 3000,
		Host: `localhost`,
	}
	if err := settings.Save(fname); err != nil {
		t.Error(err)
	}
	var result Settings
	if err := (&result).Load(fname); err != nil {
		t.Error(err)
	}
	if settings != result {
		t.Errorf(`%+v не равно %+v`, settings, result)
	}
}

func TestSettings_Load_errors(t *testing.T) {
	var s Settings
	if err := s.Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("expected read error")
	}

	badFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badFile, []byte("not-json"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := (&Settings{}).Load(badFile); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestSettings_Save_marshalError(t *testing.T) {
	old := marshalSettings
	marshalSettings = func(any, string, string) ([]byte, error) {
		return nil, os.ErrInvalid
	}
	defer func() { marshalSettings = old }()

	if err := (Settings{Port: 1, Host: "h"}).Save(filepath.Join(t.TempDir(), "s.json")); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestSettings_Save_writeError(t *testing.T) {
	old := writeSettingsFile
	writeSettingsFile = func(string, []byte, os.FileMode) error { return os.ErrInvalid }
	defer func() { writeSettingsFile = old }()

	settings := Settings{Port: 8080, Host: "localhost"}
	if err := settings.Save(filepath.Join(t.TempDir(), "settings.json")); err == nil {
		t.Fatal("expected write error")
	}
}
