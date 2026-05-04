package config

import (
	"flag"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv(envDatabaseURI, "postgres://localhost/db")
	t.Setenv(envAccrualSystemAddress, "http://accrual/")
	t.Setenv(envRunAddress, ":9999")
	t.Setenv(envJWTSecret, "secret")
	t.Setenv(envMigrationsPath, "/tmp/m")

	got, err := Load(false)
	if err != nil {
		t.Fatal(err)
	}
	if got.DatabaseURI != "postgres://localhost/db" {
		t.Fatalf("DatabaseURI: %q", got.DatabaseURI)
	}
	if got.AccrualSystemAddress != "http://accrual" {
		t.Fatalf("AccrualSystemAddress: %q", got.AccrualSystemAddress)
	}
	if got.RunAddress != ":9999" {
		t.Fatalf("RunAddress: %q", got.RunAddress)
	}
	if got.JWTSecret != "secret" {
		t.Fatalf("JWTSecret")
	}
	if got.MigrationPath != "/tmp/m" {
		t.Fatalf("MigrationPath: %q", got.MigrationPath)
	}
}

func TestLoadMissingDB(t *testing.T) {
	t.Setenv(envDatabaseURI, "")
	t.Setenv(envAccrualSystemAddress, "http://x")
	_, err := Load(false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadMissingAccrual(t *testing.T) {
	t.Setenv(envDatabaseURI, "postgres://x")
	t.Setenv(envAccrualSystemAddress, "")
	_, err := Load(false)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadDefaultJWTSecret(t *testing.T) {
	t.Setenv(envDatabaseURI, "postgres://x")
	t.Setenv(envAccrualSystemAddress, "http://x")
	t.Setenv(envJWTSecret, "")
	t.Setenv(envMigrationsPath, "/m")
	got, err := Load(false)
	if err != nil {
		t.Fatal(err)
	}
	if got.JWTSecret != defaultInsecureJWTSecret {
		t.Fatalf("jwt %q", got.JWTSecret)
	}
}

func TestLoadWithCLIArgs(t *testing.T) {
	fs := flag.NewFlagSet("gophermart-test", flag.ContinueOnError)
	t.Setenv(envDatabaseURI, "")
	t.Setenv(envAccrualSystemAddress, "")
	t.Setenv(envRunAddress, "")
	t.Setenv(envJWTSecret, "")
	t.Setenv(envMigrationsPath, "")

	args := []string{
		"gophermart",
		"-d=postgres://cli/db",
		"-r=http://accrual/cli/",
		"-a=:7777",
		"-k=cli-secret",
		"-m=/cli/migrations",
	}
	got, err := loadWithArgs(true, fs, args)
	if err != nil {
		t.Fatal(err)
	}
	if got.DatabaseURI != "postgres://cli/db" {
		t.Fatalf("db %q", got.DatabaseURI)
	}
	if got.AccrualSystemAddress != "http://accrual/cli" {
		t.Fatalf("accrual %q", got.AccrualSystemAddress)
	}
	if got.RunAddress != ":7777" || got.JWTSecret != "cli-secret" || got.MigrationPath != "/cli/migrations" {
		t.Fatalf("%+v", got)
	}
}
