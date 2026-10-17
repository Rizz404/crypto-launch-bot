// Package migrations embeds the goose SQL migrations so the bot binary can
// apply them itself (see "bot migrate").
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
