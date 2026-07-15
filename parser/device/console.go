package device

import "io/fs"

// Console ports Parser\Device\Console: console detection over consoles.yml,
// gated on the combined pre-match.
type Console struct {
	preMatchParser
}

// NewConsole loads the console regex database from fsys.
func NewConsole(fsys fs.FS) (*Console, error) {
	pm, err := newPreMatch(fsys, "console", "device/consoles.yml")
	if err != nil {
		return nil, err
	}

	return &Console{preMatchParser: pm}, nil
}
