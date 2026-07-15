// This port is a derivative work of matomo/device-detector
// (https://github.com/matomo-org/device-detector), Copyright (C) Matomo Team,
// and is licensed under LGPL-3.0-or-later.

package devicedetector

import (
	"embed"
	"io/fs"
)

//go:embed data/regexes
var embeddedData embed.FS

// EmbeddedRegexes returns the regex database bundled with the library,
// rooted so that e.g. "bots.yml" and "client/browsers.yml" resolve directly.
func EmbeddedRegexes() fs.FS {
	sub, err := fs.Sub(embeddedData, "data/regexes")
	if err != nil {
		// The embed path is fixed at compile time; failure here is unreachable.
		panic(err)
	}

	return sub
}
