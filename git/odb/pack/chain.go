package pack

type Chain interface {
	Data() ([]byte, error)
	Type() PackedObjectType
}
