package dataframe

type DataType uint8

const (
	Float64 DataType = iota
	Int64
	Bool
	String
	DateTime
	Category
)

func (t DataType) String() string {
	switch t {
	case Float64:
		return "float64"
	case Int64:
		return "int64"
	case Bool:
		return "bool"
	case String:
		return "string"
	case DateTime:
		return "datetime"
	case Category:
		return "category"
	}
	return "unknown"
}
