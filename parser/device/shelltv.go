package device

import (
	"io/fs"

	"github.com/skalibog/devicedetector/parser"
)

// shellTvRegex detects "{brand} Shell" and tclwebkit fragments
// (Parser\Device\ShellTv::isShellTv).
const shellTvRegex = `[a-z]+[ _]Shell[ _]\w{6}|tclwebkit(\d+[.\d]*)`

// ShellTv ports Parser\Device\ShellTv: TV detection over shell_tv.yml, gated on
// a shell/tclwebkit pre-check, with the device type forced to TV.
type ShellTv struct {
	base
}

// NewShellTv loads the shell TV regex database from fsys.
func NewShellTv(fsys fs.FS) (*ShellTv, error) {
	b, err := loadBase(fsys, "shelltv", "device/shell_tv.yml")
	if err != nil {
		return nil, err
	}

	return &ShellTv{base: b}, nil
}

// Parse only proceeds for ShellTv user agents and always yields a TV result.
func (p *ShellTv) Parse(ua string) (*Result, error) {
	m, err := parser.MatchUserAgent(ua, shellTvRegex)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, nil
	}

	res, err := p.parse(ua)
	if err != nil {
		return nil, err
	}

	if res == nil {
		res = &Result{Type: TypeUnknown}
	}

	res.Type = TypeTV

	return res, nil
}
