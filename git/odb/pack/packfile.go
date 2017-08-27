package pack

import (
	"io"

	"github.com/git-lfs/git-lfs/errors"
)

type Packfile struct {
	Version uint32
	Objects uint32
	Idx     *Index

	r io.ReaderAt
}

func (p *Packfile) Object(name []byte) (*Object, error) {
	entry, err := p.Idx.Entry(name)
	if err != nil {
		return nil, err
	}

	r, err := p.unpackObject(int64(entry.PackOffset))
	if err != nil {
		return nil, err
	}

	return &Object{
		data: r,
		typ:  r.Type(),
	}, nil
}

func (p *Packfile) unpackObject(offset int64) (Chain, error) {
	buf := make([]byte, 1)
	if _, err := p.r.ReadAt(buf, offset); err != nil {
		return nil, err
	}

	objectOffset := offset

	typ := PackedObjectType((buf[0] >> 4) & 0x7)
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

	switch typ {
	case TypeObjectOffsetDelta, TypeObjectReferenceDelta:
		base, offset, err := p.findBase(typ, offset, objectOffset)
		if err != nil {
			return nil, err
		}

		return &ChainDelta{
			base: base,
			delta: &ChainBase{
				offset: offset,
				size:   int64(size),

				r: p.r,
			},
		}, nil
	case TypeCommit, TypeTree, TypeBlob, TypeTag:
		return &ChainBase{
			offset: offset,
			size:   int64(size),
			typ:    typ,

			r: p.r,
		}, nil
	}
	return nil, errUnrecognizedObjectType
}

func (p *Packfile) findBase(typ PackedObjectType, offset, objOffset int64) (Chain, int64, error) {
	var baseOffset int64

	var sha [20]byte
	if _, err := p.r.ReadAt(sha[:], offset); err != nil {
		return nil, baseOffset, err
	}

	switch typ {
	case TypeObjectOffsetDelta:
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
			return nil, baseOffset, err
		}

		baseOffset = int64(e.PackOffset)
		offset += 20
	default:
		return nil, baseOffset, errors.Errorf("git/odb/pack: type %s is not deltafied", typ)
	}

	r, err := p.unpackObject(baseOffset)
	return r, offset, err
}
