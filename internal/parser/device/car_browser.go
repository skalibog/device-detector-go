package device

import "io/fs"

// CarBrowser ports Parser\Device\CarBrowser: in-car browser detection over
// car_browsers.yml, gated on the combined pre-match.
type CarBrowser struct {
	preMatchParser
}

// NewCarBrowser loads the car browser regex database from fsys.
func NewCarBrowser(fsys fs.FS) (*CarBrowser, error) {
	pm, err := newPreMatch(fsys, "car browser", "device/car_browsers.yml")
	if err != nil {
		return nil, err
	}

	return &CarBrowser{preMatchParser: pm}, nil
}
