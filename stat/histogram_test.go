package stat_test

import (
	"math"
	"slices"
	"testing"

	"github.com/axent-pl/dataframe"
	"github.com/axent-pl/dataframe/stat"
)

func column[T dataframe.Value](data []T, dtype dataframe.DataType, nulls ...int) *dataframe.TypedColumn[T] {
	c := &dataframe.TypedColumn[T]{Data: data, DType: dtype}
	if len(nulls) > 0 {
		c.Nulls = dataframe.NewBitmap(len(data))
		for _, i := range nulls {
			c.Nulls.Set(i)
		}
	}
	return c
}

func Test_Histogram(t *testing.T) {
	tests := []struct {
		name   string
		column dataframe.Column
		opts   []stat.HistogramOption
		data   []uint32
		dict   []string
		nulls  []int
	}{
		{
			name: "float range and boundaries", column: column([]float64{-1, 0, 1, 2, 3, 4, 5}, dataframe.Float64),
			opts: []stat.HistogramOption{stat.WithRange(0, 4), stat.WithBins(2)},
			data: []uint32{0, 0, 0, 1, 1, 1, 0}, dict: []string{"[0, 2)", "[2, 4]"}, nulls: []int{0, 6},
		},
		{
			name: "integer max bins", column: column([]int64{4, 0, 1, 2, 3}, dataframe.Int64),
			opts: []stat.HistogramOption{stat.WithMaxBins(2), stat.WithBins(4)},
			data: []uint32{1, 0, 0, 1, 1}, dict: []string{"[0, 2)", "[2, 4]"},
		},
		{
			name: "nonuniform edges", column: column([]float64{0, 1, 3, 4, 10}, dataframe.Float64),
			opts: []stat.HistogramOption{stat.WithBinEdges(0, 1, 4, 10)},
			data: []uint32{0, 1, 1, 2, 2}, dict: []string{"[0, 1)", "[1, 4)", "[4, 10]"},
		},
		{
			name: "constant", column: column([]float64{7, 7, 7}, dataframe.Float64),
			data: []uint32{0, 0, 0}, dict: []string{"[7, 7]"},
		},
		{
			name: "nulls and nonfinite values", column: column([]float64{100, math.NaN(), math.Inf(-1), math.Inf(1), -2, 0, 2}, dataframe.Float64, 0),
			opts: []stat.HistogramOption{stat.WithBins(2)},
			data: []uint32{0, 0, 0, 0, 0, 1, 1}, dict: []string{"[-2, 0)", "[0, 2]"}, nulls: []int{0, 1, 2, 3},
		},
		{
			name: "all invalid", column: column([]float64{0, math.NaN(), math.Inf(1)}, dataframe.Float64, 0),
			data: []uint32{0, 0, 0}, nulls: []int{0, 1, 2},
		},
		{
			name: "empty numeric", column: column([]float64{}, dataframe.Float64),
		},
		{
			name: "empty explicit range", column: column([]float64{}, dataframe.Float64),
			opts: []stat.HistogramOption{stat.WithRange(0, 4), stat.WithBins(2)},
			dict: []string{"[0, 2)", "[2, 4]"},
		},
		{
			name: "empty explicit edges", column: column([]int64{}, dataframe.Int64),
			opts: []stat.HistogramOption{stat.WithBinEdges(-1, 1)}, dict: []string{"[-1, 1]"},
		},
		{
			name: "finite range with overflowing width", column: column([]float64{-math.MaxFloat64, 0, math.MaxFloat64}, dataframe.Float64),
			opts: []stat.HistogramOption{stat.WithBins(2)}, data: []uint32{0, 1, 1},
			dict: []string{"[-1.7976931348623157e+308, 0)", "[0, 1.7976931348623157e+308]"},
		},
		{
			name: "rounded edges collapse", column: column([]float64{1, math.Nextafter(1, 2)}, dataframe.Float64),
			data: []uint32{0, 0}, dict: []string{"[1, 1.0000000000000002]"},
		},
		{
			name: "bool", column: column([]bool{true, false, true, false}, dataframe.Bool, 3),
			data: []uint32{0, 1, 0, 0}, dict: []string{"true", "false"}, nulls: []int{3},
		},
		{
			name: "string", column: column([]string{"b", "a", "b", "", "ignored"}, dataframe.String, 4),
			data: []uint32{0, 1, 0, 2, 0}, dict: []string{"b", "a", ""}, nulls: []int{4},
		},
		{
			name: "category labels and unused dictionary entries",
			column: &dataframe.CategoryColumn{
				TypedColumn: *column([]uint32{2, 1, 0, 99}, dataframe.Category, 3),
				Dictionary:  []string{"a", "b", "a", "unused"},
			},
			data: []uint32{0, 1, 0, 0}, dict: []string{"a", "b"}, nulls: []int{3},
		},
		{
			name: "all null string", column: column([]string{"x", "y"}, dataframe.String, 0, 1),
			data: []uint32{0, 0}, nulls: []int{0, 1},
		},
		{
			name: "discrete max bins", column: column([]string{"a", "b", "a"}, dataframe.String),
			opts: []stat.HistogramOption{stat.WithMaxBins(2)}, data: []uint32{0, 1, 0}, dict: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stat.Histogram(tt.column, tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if got.Type() != dataframe.Category || got.Len() != tt.column.Len() {
				t.Fatalf("got type %s and length %d", got.Type(), got.Len())
			}
			if !slices.Equal(got.Data, tt.data) || !slices.Equal(got.Dictionary, tt.dict) {
				t.Fatalf("got data %v, dictionary %v; want %v, %v", got.Data, got.Dictionary, tt.data, tt.dict)
			}
			for i := range got.Data {
				if got.IsNull(i) != slices.Contains(tt.nulls, i) {
					t.Errorf("row %d: null = %v", i, got.IsNull(i))
				}
			}
		})
	}
}

func Test_Histogram_DefaultBins(t *testing.T) {
	got, err := stat.Histogram(column([]float64{0, 5, 10}, dataframe.Float64))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Dictionary) != 10 || !slices.Equal(got.Data, []uint32{0, 5, 9}) {
		t.Fatalf("unexpected default bins: %+v", got)
	}
}

func Test_Histogram_Errors(t *testing.T) {
	numeric := column([]float64{0, 1}, dataframe.Float64)
	text := column([]string{"a", "b"}, dataframe.String)
	tests := []struct {
		name   string
		column dataframe.Column
		opts   []stat.HistogramOption
	}{
		{"nil column", nil, nil},
		{"typed nil", (*dataframe.TypedColumn[float64])(nil), nil},
		{"nil option", numeric, []stat.HistogramOption{nil}},
		{"zero bins", numeric, []stat.HistogramOption{stat.WithBins(0)}},
		{"negative max bins", numeric, []stat.HistogramOption{stat.WithMaxBins(-1)}},
		{"reversed range", numeric, []stat.HistogramOption{stat.WithRange(2, 1)}},
		{"constant range", numeric, []stat.HistogramOption{stat.WithRange(1, 1)}},
		{"nonfinite range", numeric, []stat.HistogramOption{stat.WithRange(0, math.Inf(1))}},
		{"NaN range", numeric, []stat.HistogramOption{stat.WithRange(math.NaN(), 1)}},
		{"no edges", numeric, []stat.HistogramOption{stat.WithBinEdges()}},
		{"single edge", numeric, []stat.HistogramOption{stat.WithBinEdges(1)}},
		{"duplicate edges", numeric, []stat.HistogramOption{stat.WithBinEdges(0, 1, 1)}},
		{"unsorted edges", numeric, []stat.HistogramOption{stat.WithBinEdges(0, 2, 1)}},
		{"NaN edge", numeric, []stat.HistogramOption{stat.WithBinEdges(0, math.NaN())}},
		{"infinite edge", numeric, []stat.HistogramOption{stat.WithBinEdges(0, math.Inf(1))}},
		{"edges and bins", numeric, []stat.HistogramOption{stat.WithBinEdges(0, 1), stat.WithBins(1)}},
		{"edges and range", numeric, []stat.HistogramOption{stat.WithBinEdges(0, 1), stat.WithRange(0, 1)}},
		{"edges exceed cap", numeric, []stat.HistogramOption{stat.WithBinEdges(0, 1, 2), stat.WithMaxBins(1)}},
		{"labels exceed cap", text, []stat.HistogramOption{stat.WithMaxBins(1)}},
		{"discrete bins", text, []stat.HistogramOption{stat.WithBins(2)}},
		{"discrete range", text, []stat.HistogramOption{stat.WithRange(0, 1)}},
		{"discrete edges", text, []stat.HistogramOption{stat.WithBinEdges(0, 1)}},
		{"unsupported datetime", column([]int64{1}, dataframe.DateTime), nil},
		{"mismatched type", column([]string{"a"}, dataframe.Float64), nil},
		{"category without dictionary", column([]uint32{0}, dataframe.Category), nil},
		{"invalid category ID", &dataframe.CategoryColumn{
			TypedColumn: *column([]uint32{1}, dataframe.Category), Dictionary: []string{"a"},
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := stat.Histogram(tt.column, tt.opts...); err == nil || got != nil {
				t.Fatalf("got %v, %v; want nil result and an error", got, err)
			}
		})
	}
}
