package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	content := `{
		"server_address": "localhost:9090",
		"base_url": "http://localhost",
		"file_storage_path": "/tmp/links.json",
		"enable_https": true
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if got.ServerAddress == nil || *got.ServerAddress != "localhost:9090" {
		t.Errorf("ServerAddress = %v; want localhost:9090", got.ServerAddress)
	}
	if got.BaseURL == nil || *got.BaseURL != "http://localhost" {
		t.Errorf("BaseURL = %v; want http://localhost", got.BaseURL)
	}
	if got.FileStoragePath == nil || *got.FileStoragePath != "/tmp/links.json" {
		t.Errorf("FileStoragePath = %v; want /tmp/links.json", got.FileStoragePath)
	}
	if got.EnableHTTPS == nil || !*got.EnableHTTPS {
		t.Errorf("EnableHTTPS = %v; want true", got.EnableHTTPS)
	}
	if got.DatabaseDSN != nil {
		t.Errorf("DatabaseDSN = %v; want nil", got.DatabaseDSN)
	}
}

func TestLoadFile_missingFile(t *testing.T) {
	if _, err := LoadFile("/nonexistent/config.json"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("expected parse error")
	}
}
