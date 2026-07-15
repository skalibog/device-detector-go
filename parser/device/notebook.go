package device

import (
	"io/fs"

	"github.com/skalibog/devicedetector/parser"
)

// Notebook ports Parser\Device\Notebook: notebook detection in Facebook user
// agents, gated on the FBMD/ fragment.
type Notebook struct {
	base
}

// NewNotebook loads the notebook regex database from fsys.
func NewNotebook(fsys fs.FS) (*Notebook, error) {
	b, err := loadBase(fsys, "notebook", "device/notebooks.yml")
	if err != nil {
		return nil, err
	}

	return &Notebook{base: b}, nil
}

// Parse only proceeds when the user agent carries the FBMD/ fragment.
func (p *Notebook) Parse(ua string) (*Result, error) {
	m, err := parser.MatchUserAgent(ua, "FBMD/")
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	return p.parse(ua)
}
