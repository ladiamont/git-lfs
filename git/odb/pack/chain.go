package pack

type Chain interface {
	Unpack() ([]byte, error)
	Type() PackedObjectType
}
