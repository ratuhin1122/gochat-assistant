// Package migrations contains embedded SQL schema migrations.
package migrations

import "embed"

// Files embeds all SQL migration files in this directory.
//
//go:embed *.sql
var Files embed.FS
