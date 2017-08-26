package pack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/git-lfs/git-lfs/errors"
)

type Packfile struct {
	Version uint32
	Objects uint32
	Idx     *Index

	r io.ReaderAt
}

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

type PackedObjectType uint8

const (
	TypeNone PackedObjectType = iota
	TypeCommit
	TypeTree
	TypeBlob
	TypeTag

	TypeObjectOffsetDelta    PackedObjectType = 6
	TypeObjectReferenceDelta PackedObjectType = 7
)

func (t PackedObjectType) String() string {
	switch t {
	case TypeNone:
		return "<none>"
	case TypeCommit:
		return "commit"
	case TypeTree:
		return "tree"
	case TypeBlob:
		return "blob"
	case TypeTag:
		return "tag"
	case TypeObjectOffsetDelta:
		return "obj_ofs_delta"
	case TypeObjectReferenceDelta:
		return "obf_ref_delta"
	}
	panic(fmt.Sprintf("git/odb/pack: unknown object type: %d", t))
}

var (
	errUnrecognizedObjectType = errors.New("git/odb/pack: unrecognized object type")
)

func (p *Packfile) Object(name []byte) ([]byte, error) {
	entry, err := p.Idx.Entry(name)
	if err != nil {
		return nil, err
	}

	return p.unpackObject(int64(entry.PackOffset))
}

func (p *Packfile) unpackObject(offset int64) ([]byte, error) {
	buf := make([]byte, 1)
	if _, err := p.r.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	objectOffset := offset

	typ := (buf[0] >> 4) & 0x7
	size := uint64(buf[0] & 0xf)
	shift := uint(4)
	offset += 1

	for buf[0]&0x80 != 0 {
		if _, err := p.r.ReadAt(buf, offset); err != nil {
			return nil, err
		}

		size |= (uint64(buf[0]&0x7f) << shift)
		shift += 7
		offset += 1
	}

	switch PackedObjectType(typ) {
	case TypeObjectOffsetDelta, TypeObjectReferenceDelta:
		return p.unpackDeltafied(PackedObjectType(typ), offset, objectOffset, int64(size))
	case TypeCommit, TypeTree, TypeBlob:
		return p.unpackCompressed(offset, int64(size))
	default:
		return nil, errUnrecognizedObjectType
	}
	return nil, nil
}

func (p *Packfile) unpackCompressed(offset, size int64) ([]byte, error) {
	zr, err := zlib.NewReader(&OffsetReader{
		r: p.r,
		o: offset,
	})

	if err != nil {
		return nil, err
	}

	defer zr.Close()

	buf := make([]byte, size)
	if _, err := io.ReadFull(zr, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (p *Packfile) unpackDeltafied(typ PackedObjectType, offset, objOffset, size int64) ([]byte, error) {
	var baseOffset int64

	var sha [20]byte
	if _, err := p.r.ReadAt(sha[:], offset); err != nil {
		return nil, err
	}

	switch typ {
	case TypeObjectOffsetDelta:
		// i = 0
		// c = data.getord(i)
		// base_offset = c & 0x7f
		// while c & 0x80 != 0
		//   c = data.getord(i += 1)
		//   base_offset += 1
		//   base_offset <<= 7
		//   base_offset |= c & 0x7f
		// end
		// base_offset = obj_offset - base_offset
		// offset += i + 1

		i := 0
		c := int64(sha[i])
		baseOffset = c & 0x7f

		for c&0x80 != 0 {
			i += 1
			c = int64(sha[i])

			baseOffset += 1
			baseOffset <<= 7
			baseOffset |= c & 0x7f
		}

		baseOffset = objOffset - baseOffset
		offset += int64(i) + 1
	case TypeObjectReferenceDelta:
		e, err := p.Idx.Entry(sha[:])
		if err != nil {
			return nil, err
		}

		baseOffset = int64(e.PackOffset)
		offset += 20
	default:
		return nil, errors.Errorf("git/odb/pack: type %s is not deltafied", typ)
	}

	base, err := p.unpackObject(baseOffset)
	if err != nil {
		return nil, err
	}

	delta, err := p.unpackCompressed(offset, size)
	if err != nil {
		return nil, err
	}

	return patch(base, delta)
}

func patch(base, delta []byte) ([]byte, error) {
	srcSize, pos := patchDeltaHeader(delta, 0)
	if srcSize != int64(len(base)) {
		return nil, errors.New("git/odb/pack: invalid delta data")
	}

	var dest []byte

	destSize, pos := patchDeltaHeader(delta, pos)

	for pos < len(delta) {
		c := int(delta[pos])
		pos += 1

		if c&0x80 != 0 {
			pos -= 1

			var co, cs int

			if c&0x1 != 0 {
				pos += 1
				co = int(delta[pos])
			}
			if c&0x2 != 0 {
				pos += 1
				co |= (int(delta[pos]) << 8)
			}
			if c&0x4 != 0 {
				pos += 1
				co |= (int(delta[pos]) << 16)
			}
			if c&0x8 != 0 {
				pos += 1
				co |= (int(delta[pos]) << 24)
			}

			if c&0x10 != 0 {
				pos += 1
				cs = int(delta[pos])
			}
			if c&0x20 != 0 {
				pos += 1
				cs |= (int(delta[pos]) << 8)
			}
			if c&0x40 != 0 {
				pos += 1
				cs |= (int(delta[pos]) << 16)
			}

			if cs == 0 {
				cs = 0x10000
			}
			pos += 1

			dest = append(dest, base[co:co+cs]...)
		} else if c != 0 {
			dest = append(dest, delta[pos:int(pos)+c]...)
			pos += int(c)
		} else {
			return nil, errors.New("git/odb/pack: invalid delta data")
		}
	}

	if destSize != int64(len(dest)) {
		return nil, errors.New("git/odb/pack: invalid delta data")
	}
	return dest, nil
}

func patchDeltaHeader(delta []byte, pos int) (size int64, end int) {
	var shift uint
	var c int64

	for shift == 0 || c&0x80 != 0 {
		if len(delta) <= pos {
			panic("git/odb/pack: invalid delta header")
		}

		c = int64(delta[pos])

		pos++
		size |= (c & 0x7f) << shift
		shift += 7
	}

	return size, pos
}

type OffsetReader struct {
	r io.ReaderAt
	o int64
}

func (r *OffsetReader) Read(p []byte) (n int, err error) {
	n, err = r.r.ReadAt(p, r.o)
	r.o += int64(n)

	return n, err
}
