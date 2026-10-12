package dataframe

type Value interface {
	~float64 | ~int64 | ~bool | ~string | ~uint32
}
type Column interface {
	Type() DataType
	Len() int
	IsNull(int) bool
}

type TypedColumn[T Value] struct {
	Data  []T
	Nulls *Bitmap
	DType DataType
}

func (c *TypedColumn[T]) Type() DataType    { return c.DType }
func (c *TypedColumn[T]) Len() int          { return len(c.Data) }
func (c *TypedColumn[T]) IsNull(i int) bool { return c.Nulls.Get(i) }

type CategoryColumn struct {
	TypedColumn[uint32]
	Dictionary []string
}
