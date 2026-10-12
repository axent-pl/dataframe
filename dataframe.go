package dataframe

import "fmt"

type ColumnSpec struct {
	Name     string
	Type     DataType
	Nullable bool
}

type Schema struct {
	Columns     []ColumnSpec
	ColumnIndex map[string]int
}

type DataFrame struct {
	Schema  Schema
	Columns []Column
	NumRows int
}

func NewDataFrame(fields []ColumnSpec, columns []Column) (*DataFrame, error) {
	if len(fields) != len(columns) {
		return nil, fmt.Errorf("fields/columns mismatch")
	}
	index := make(map[string]int, len(fields))
	n := 0
	for i, f := range fields {
		if f.Name == "" {
			return nil, fmt.Errorf("empty column name")
		}
		if _, ok := index[f.Name]; ok {
			return nil, fmt.Errorf("duplicate column %q", f.Name)
		}
		index[f.Name] = i
		if columns[i] == nil || columns[i].Type() != f.Type {
			return nil, fmt.Errorf("column %q type mismatch", f.Name)
		}
		if i == 0 {
			n = columns[i].Len()
		} else if columns[i].Len() != n {
			return nil, fmt.Errorf("column %q length mismatch", f.Name)
		}
	}
	return &DataFrame{Schema: Schema{Columns: append([]ColumnSpec(nil), fields...), ColumnIndex: index}, Columns: append([]Column(nil), columns...), NumRows: n}, nil
}

func (df *DataFrame) AddColumn(spec ColumnSpec, column Column) error {
	if spec.Name == "" {
		return fmt.Errorf("empty column name")
	}
	if _, ok := df.Schema.ColumnIndex[spec.Name]; ok {
		return fmt.Errorf("duplicate column %q", spec.Name)
	}
	if column == nil || column.Type() != spec.Type {
		return fmt.Errorf("column %q type mismatch", spec.Name)
	}
	n := column.Len()
	if len(df.Columns) > 0 && n != df.NumRows {
		return fmt.Errorf("column %q length mismatch", spec.Name)
	}

	if df.Schema.ColumnIndex == nil {
		df.Schema.ColumnIndex = make(map[string]int)
	}
	if len(df.Columns) == 0 {
		df.NumRows = n
	}
	df.Schema.ColumnIndex[spec.Name] = len(df.Columns)
	df.Schema.Columns = append(df.Schema.Columns, spec)
	df.Columns = append(df.Columns, column)
	return nil
}

func (df *DataFrame) Column(name string) (Column, error) {
	i, ok := df.Schema.ColumnIndex[name]
	if !ok {
		return nil, fmt.Errorf("unknown column %q", name)
	}
	return df.Columns[i], nil
}
