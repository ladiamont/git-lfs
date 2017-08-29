package pack

import (
	"compress/zlib"
	"io"
)

type ChainBase struct {
	offset int64
	size   int64
	typ    PackedObjectType

	r io.ReaderAt
}

func (b *ChainBase) Unpack() ([]byte, error) {
	zr, err := zlib.NewReader(&OffsetReaderAt{
		r: b.r,
		o: b.offset,
	})

	if err != nil {
		return nil, err
	}

	defer zr.Close()

	buf := make([]byte, b.size)
	if _, err := io.ReadFull(zr, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (b *ChainBase) Type() PackedObjectType {
	return b.typ
}
