package device

import "io/fs"

// Mobile ports Parser\Device\Mobile: the generic device parser over
// mobiles.yml, with no pre-check.
type Mobile struct {
	base
}

// NewMobile loads the mobile regex database from fsys.
func NewMobile(fsys fs.FS) (*Mobile, error) {
	b, err := loadBase(fsys, "mobile", "device/mobiles.yml")
	if err != nil {
		return nil, err
	}

	return &Mobile{base: b}, nil
}

// Parse runs the generic device flow directly.
func (p *Mobile) Parse(ua string) (*Result, error) {
	return p.parse(ua)
}
