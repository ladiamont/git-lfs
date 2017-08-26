package pack

type Object struct {
	typ  PackedObjectType
	data Chain
}

func (o *Object) Data() ([]byte, error) {
	return o.data.Data()
}

func (o *Object) Type() PackedObjectType { return o.typ }
