package device

import "io/fs"

// Camera ports Parser\Device\Camera: camera detection over cameras.yml, gated
// on the combined pre-match.
type Camera struct {
	preMatchParser
}

// NewCamera loads the camera regex database from fsys.
func NewCamera(fsys fs.FS) (*Camera, error) {
	pm, err := newPreMatch(fsys, "camera", "device/cameras.yml")
	if err != nil {
		return nil, err
	}

	return &Camera{preMatchParser: pm}, nil
}
