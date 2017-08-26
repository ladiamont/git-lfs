package pack

import (
	"errors"
	"fmt"
)

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
