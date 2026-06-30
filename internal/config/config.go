// Package config loads shortener settings from a JSON configuration file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// File holds optional settings read from a JSON configuration file.
type File struct {
	ServerAddress   *string `json:"server_address"`
	BaseURL         *string `json:"base_url"`
	FileStoragePath *string `json:"file_storage_path"`
	DatabaseDSN     *string `json:"database_dsn"`
	MigrationsPath  *string `json:"migrations_path"`
	SecretKey       *string `json:"secret_key"`
	AuditFile       *string `json:"audit_file"`
	AuditURL        *string `json:"audit_url"`
	EnableHTTPS     *bool   `json:"enable_https"`
}

// LoadFile reads and parses a JSON configuration file at path.
func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, fmt.Errorf("read config file: %w", err)
	}
	var cfg File
	if err := json.Unmarshal(data, &cfg); err != nil {
		return File{}, fmt.Errorf("parse config file: %w", err)
	}
	return cfg, nil
}
