package pack

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"golang.org/x/exp/mmap"
)

type Set struct {
	m       map[byte][]*Packfile
	closeFn func() error
}

var (
	nameRe = regexp.MustCompile(`pack-([a-f0-9]{40}).pack$`)
)

func NewSet() (*Set, error) { return NewSetRoot("") }

func NewSetRoot(db string) (*Set, error) {
	pd := filepath.Join(db, "pack")

	paths, err := filepath.Glob(filepath.Join(pd, "pack-*.pack"))
	if err != nil {
		return nil, err
	}

	packs := make([]*Packfile, 0, len(paths))

	for _, path := range paths {
		submatch := nameRe.FindStringSubmatch(path)
		if len(submatch) != 2 {
			continue
		}

		name := submatch[1]

		packf, err := mmap.Open(filepath.Join(pd, fmt.Sprintf("pack-%s.pack", name)))
		if err != nil {
			return nil, err
		}

		idxf, err := mmap.Open(filepath.Join(pd, fmt.Sprintf("pack-%s.idx", name)))
		if err != nil {
			return nil, err
		}

		pack, err := DecodePackfile(packf)
		if err != nil {
			return nil, err
		}

		idx, err := DecodeIndex(idxf)
		if err != nil {
			return nil, err
		}

		pack.idx = idx

		packs = append(packs, pack)
	}

	m := make(map[byte][]*Packfile)

	for i := 0; i < 256; i++ {
		n := byte(i)

		for _, pack := range packs {
			if pack.idx.fanout[n] == 0 {
				continue
			}
			m[n] = append(m[n], pack)
		}

		sort.Slice(m[n], func(i, j int) bool {
			ni := m[n][i].idx.fanout[n]
			nj := m[n][j].idx.fanout[n]

			return ni > nj
		})
	}

	return &Set{
		m: m,

		closeFn: func() error {
			for _, pack := range packs {
				if err := pack.Close(); err != nil {
					return err
				}
			}
			return nil
		},
	}, nil
}

func (s *Set) Object(name []byte) (*Object, error) {
	var key byte
	if len(name) > 0 {
		key = name[0]
	}

	for _, pack := range s.m[key] {
		o, err := pack.Object(name)
		if err != nil {
			if IsNotFound(err) {
				continue
			}
			return nil, err
		}
		return o, nil
	}
	return nil, nil
}

func (s *Set) Close() error {
	if s.closeFn == nil {
		return nil
	}
	return s.closeFn()
}
