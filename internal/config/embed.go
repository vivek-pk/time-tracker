//go:build !debug

package config

import _ "embed"

// embeddedConfigJSON is the production config baked into the binary at build time.
//
//go:embed config.json
var embeddedConfigJSON []byte
