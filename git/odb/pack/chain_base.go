package pack

import (
	"compress/zlib"
	"fmt"
	"io"
	"runtime"
)

type ChainBase struct {
	offset int64
	size   int64
	typ    PackedObjectType

	r io.ReaderAt
}

func (b *ChainBase) Data() ([]byte, error) {
	zr, err := zlib.NewReader(&OffsetReaderAt{
		r: b.r,
		o: b.offset,
	})

	if err != nil {
		for i := 0; i < 8; i++ {
			_, f, l, ok := runtime.Caller(i)
			if ok {
				fmt.Printf("%s:%d\n", f, l)
			}
		}
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
