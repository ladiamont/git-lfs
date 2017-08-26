package pack

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

var (
	packHeader = []byte{'P', 'A', 'C', 'K'}

	errBadPackHeader = errors.New("git/odb/pack: bad pack header")
)

func DecodePackfile(r io.ReaderAt) (*Packfile, error) {
	header := make([]byte, 12)
	if _, err := r.ReadAt(header[:], 0); err != nil {
		return nil, err
	}

	if !bytes.HasPrefix(header, packHeader) {
		return nil, errBadPackHeader
	}

	version := binary.BigEndian.Uint32(header[4:])
	objects := binary.BigEndian.Uint32(header[8:])

	return &Packfile{
		Version: version,
		Objects: objects,

		r: r,
	}, nil
}
