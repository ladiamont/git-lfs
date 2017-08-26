package pack

import "io"

type OffsetReaderAt struct {
	r io.ReaderAt

	o int64
}

func (r *OffsetReaderAt) Read(p []byte) (n int, err error) {
	n, err = r.r.ReadAt(p, r.o)
	r.o += int64(n)

	return n, err
}
