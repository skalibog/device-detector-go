package device

import (
	"io/fs"

	"github.com/skalibog/devicedetector/parser"
)

// hbbTvRegex detects HbbTV / SmartTvA fragments (Parser\Device\HbbTv::isHbbTv).
const hbbTvRegex = `(?:HbbTV|SmartTvA)/([1-9]{1}(?:\.[0-9]{1}){1,2})`

// HbbTv ports Parser\Device\HbbTv: TV detection over televisions.yml, gated on
// an HbbTV/SmartTvA pre-check, defaulting the device type to TV.
type HbbTv struct {
	base
}

// NewHbbTv loads the television regex database from fsys.
func NewHbbTv(fsys fs.FS) (*HbbTv, error) {
	b, err := loadBase(fsys, "tv", "device/televisions.yml")
	if err != nil {
		return nil, err
	}

	return &HbbTv{base: b}, nil
}

// Parse only proceeds for HbbTV/SmartTvA user agents and always yields a TV
// result, even when no brand or model could be resolved.
func (p *HbbTv) Parse(ua string) (*Result, error) {
	m, err := parser.MatchUserAgent(ua, hbbTvRegex)
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

	if res.Type == TypeUnknown {
		res.Type = TypeTV
	}

	return res, nil
}
