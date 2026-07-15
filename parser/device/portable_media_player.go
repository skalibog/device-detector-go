package device

import "io/fs"

// PortableMediaPlayer ports Parser\Device\PortableMediaPlayer: portable media
// player detection over portable_media_player.yml, gated on the combined
// pre-match.
type PortableMediaPlayer struct {
	preMatchParser
}

// NewPortableMediaPlayer loads the portable media player regex database from fsys.
func NewPortableMediaPlayer(fsys fs.FS) (*PortableMediaPlayer, error) {
	pm, err := newPreMatch(fsys, "portablemediaplayer", "device/portable_media_player.yml")
	if err != nil {
		return nil, err
	}

	return &PortableMediaPlayer{preMatchParser: pm}, nil
}
