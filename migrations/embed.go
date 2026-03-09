package migrations

import "embed"

// FS holds migration SQL files for use with golang-migrate.
//
//go:embed *.sql
var FS embed.FS
