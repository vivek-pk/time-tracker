//go:build debug

package config

import _ "embed"

// embeddedConfigJSON is the debug config baked into the binary when built with -tags debug.
//
//go:embed config.debug.json
var embeddedConfigJSON []byte
