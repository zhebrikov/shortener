// Package config loads Gophermart service settings from command-line flags and environment variables.
package config

import (
	"errors"
	"flag"
	"os"
	"strings"
)

// Settings holds runtime configuration for the loyalty HTTP server and its dependencies.
type Settings struct {
	// RunAddress is the host:port (or :port) where the HTTP server listens (RUN_ADDRESS / -a).
	RunAddress string
	// DatabaseURI is the PostgreSQL connection string (DATABASE_URI / -d).
	DatabaseURI string
	// AccrualSystemAddress is the base URL of the external accrual system, without a trailing path (ACCRUAL_SYSTEM_ADDRESS / -r).
	AccrualSystemAddress string
	// JWTSecret is the symmetric key used to sign session tokens (JWT_SECRET / -k; empty uses a dev-only default).
	JWTSecret string
	// MigrationPath is the filesystem path to SQL migration files for golang-migrate.
	MigrationPath string
}

const (
	envRunAddress            = "RUN_ADDRESS"
	envDatabaseURI           = "DATABASE_URI"
	envAccrualSystemAddress  = "ACCRUAL_SYSTEM_ADDRESS"
	envJWTSecret             = "JWT_SECRET"
	envMigrationsPath        = "MIGRATIONS_PATH"
	defaultRunAddress        = ":8080"
	defaultMigrationRel      = "migrations/gophermart"
	defaultInsecureJWTSecret = "gophermart-dev-secret-change-me"
)

// Load parses command-line flags (when parseFlags is true), applies environment variable overrides, and returns Settings.
func Load(parseFlags bool) (Settings, error) {
	return loadWithArgs(parseFlags, flag.CommandLine, os.Args)
}

func loadWithArgs(parseFlags bool, fs *flag.FlagSet, args []string) (Settings, error) {
	var (
		a = defaultRunAddress
		d string
		r string
		k string
		m = defaultMigrationRel
	)
	if parseFlags {
		fs.StringVar(&a, "a", defaultRunAddress, "HTTP server address (RUN_ADDRESS)")
		fs.StringVar(&d, "d", "", "PostgreSQL URI (DATABASE_URI)")
		fs.StringVar(&r, "r", "", "Accrual system base URL (ACCRUAL_SYSTEM_ADDRESS)")
		fs.StringVar(&k, "k", "", "JWT signing secret (JWT_SECRET)")
		fs.StringVar(&m, "m", defaultMigrationRel, "Path to migrations directory (MIGRATIONS_PATH)")
		_ = fs.Parse(args[1:])
	}

	runAddr := firstNonEmpty(os.Getenv(envRunAddress), a)
	dbURI := firstNonEmpty(os.Getenv(envDatabaseURI), d)
	accrual := firstNonEmpty(os.Getenv(envAccrualSystemAddress), r)
	secret := firstNonEmpty(os.Getenv(envJWTSecret), k)
	mig := firstNonEmpty(os.Getenv(envMigrationsPath), m)

	if dbURI == "" {
		return Settings{}, errors.New("DATABASE_URI (or -d) is required")
	}
	if accrual == "" {
		return Settings{}, errors.New("ACCRUAL_SYSTEM_ADDRESS (or -r) is required")
	}
	if secret == "" {
		secret = defaultInsecureJWTSecret
	}

	return Settings{
		RunAddress:           runAddr,
		DatabaseURI:          dbURI,
		AccrualSystemAddress: trimTrailingSlash(accrual),
		JWTSecret:            secret,
		MigrationPath:        mig,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func trimTrailingSlash(s string) string {
	for len(s) > 1 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
