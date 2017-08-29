package pack

type Object struct {
	typ  PackedObjectType
	data Chain
}

func (o *Object) Unpack() ([]byte, error) {
	return o.data.Unpack()
}

func (o *Object) Type() PackedObjectType { return o.typ }
