package pack

import "github.com/git-lfs/git-lfs/errors"

type ChainDelta struct {
	base, delta Chain
}

func (d *ChainDelta) Unpack() ([]byte, error) {
	base, err := d.base.Unpack()
	if err != nil {
		return nil, err
	}

	delta, err := d.delta.Unpack()
	if err != nil {
		return nil, err
	}

	return patch(base, delta)
}

func (d *ChainDelta) Type() PackedObjectType {
	return d.base.Type()
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
