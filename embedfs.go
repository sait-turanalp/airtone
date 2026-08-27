// Package airtone embeds the proven audio engine (shell scripts, the Swift
// system-tap capture helper, and the snapserver config template) so the
// compiled binary is fully self-contained and needs no repo checkout at runtime.
package airtone

import "embed"

// Engine holds the scripts/ and assets/ trees, extracted to the data dir at runtime.
//
//go:embed scripts assets
var Engine embed.FS
