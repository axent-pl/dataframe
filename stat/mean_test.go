package stat_test

import (
	"math"
	"strings"
	"testing"

	"github.com/axent-pl/dataframe"
	"github.com/axent-pl/dataframe/stat"
)

func Test_Mean(t *testing.T) {
	tests := []struct {
		name    string
		columns []dataframe.Column
		opts    []stat.MeanOption
		want    float64
	}{
		{
			name: "float", columns: []dataframe.Column{column([]float64{1, 2, 6}, dataframe.Float64)}, want: 3,
		},
		{
			name: "integer", columns: []dataframe.Column{column([]int64{-2, 1, 2}, dataframe.Int64)}, want: 1.0 / 3,
		},
		{
			name: "mixed types and unequal lengths",
			columns: []dataframe.Column{
				column([]float64{1, 3, 5}, dataframe.Float64), column([]int64{11}, dataframe.Int64),
			},
			want: 5,
		},
		{
			name: "skip nulls across columns",
			columns: []dataframe.Column{
				column([]float64{100, 2, math.NaN()}, dataframe.Float64, 0, 2),
				column([]int64{4, 200}, dataframe.Int64, 1),
			},
			want: 3,
		},
		{
			name: "skip nonfinite",
			columns: []dataframe.Column{
				column([]float64{math.NaN(), 2, math.Inf(-1), 4, math.Inf(1)}, dataframe.Float64),
			},
			opts: []stat.MeanOption{stat.WithSkipNonFinite(true)}, want: 3,
		},
		{
			name: "empty and all-null columns",
			columns: []dataframe.Column{
				column([]float64{}, dataframe.Float64), column([]int64{99}, dataframe.Int64, 0),
				column([]float64{2}, dataframe.Float64),
			},
			want: 2,
		},
		{
			name: "strict nulls with valid data", columns: []dataframe.Column{column([]int64{1, 2}, dataframe.Int64)},
			opts: []stat.MeanOption{stat.WithSkipNulls(false)}, want: 1.5,
		},
		{
			name: "last option wins", columns: []dataframe.Column{column([]int64{99, 2}, dataframe.Int64, 0)},
			opts: []stat.MeanOption{stat.WithSkipNulls(false), stat.WithSkipNulls(true)}, want: 2,
		},
		{
			name: "large integers do not overflow", columns: []dataframe.Column{column([]int64{math.MaxInt64, math.MaxInt64}, dataframe.Int64)},
			want: float64(math.MaxInt64),
		},
		{
			name: "overflowing float sum", columns: []dataframe.Column{column([]float64{math.MaxFloat64, math.MaxFloat64}, dataframe.Float64)},
			want: math.MaxFloat64,
		},
		{
			name:    "overflow followed by cancellation",
			columns: []dataframe.Column{column([]float64{math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}, dataframe.Float64)},
			want:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stat.Mean(tt.columns, tt.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %g, want %g", got, tt.want)
			}
		})
	}
}

func Test_Mean_Errors(t *testing.T) {
	numeric := []dataframe.Column{column([]float64{1}, dataframe.Float64)}
	tests := []struct {
		name    string
		columns []dataframe.Column
		opts    []stat.MeanOption
		message string
	}{
		{"no columns", nil, nil, "at least one"},
		{"empty column", []dataframe.Column{column([]int64{}, dataframe.Int64)}, nil, "at least one"},
		{"all null", []dataframe.Column{column([]float64{2}, dataframe.Float64, 0)}, nil, "at least one"},
		{"all skipped", []dataframe.Column{column([]float64{math.NaN(), math.Inf(1)}, dataframe.Float64)}, []stat.MeanOption{stat.WithSkipNonFinite(true)}, "at least one"},
		{"nil column", []dataframe.Column{numeric[0], nil}, nil, "index 1"},
		{"typed nil", []dataframe.Column{(*dataframe.TypedColumn[int64])(nil)}, nil, "nil mean column"},
		{"nil option", numeric, []stat.MeanOption{nil}, "nil mean option"},
		{"strict null", []dataframe.Column{numeric[0], column([]int64{0, 2}, dataframe.Int64, 1)}, []stat.MeanOption{stat.WithSkipNulls(false)}, "column 1, row 1"},
		{"NaN", []dataframe.Column{column([]float64{math.NaN()}, dataframe.Float64)}, nil, "non-finite"},
		{"positive infinity", []dataframe.Column{column([]float64{math.Inf(1)}, dataframe.Float64)}, nil, "non-finite"},
		{"negative infinity", []dataframe.Column{column([]float64{math.Inf(-1)}, dataframe.Float64)}, nil, "non-finite"},
		{"nonfinite option reset", []dataframe.Column{column([]float64{math.NaN()}, dataframe.Float64)}, []stat.MeanOption{stat.WithSkipNonFinite(true), stat.WithSkipNonFinite(false)}, "non-finite"},
		{"bool", []dataframe.Column{column([]bool{true}, dataframe.Bool)}, nil, "unsupported"},
		{"string", []dataframe.Column{column([]string{"1"}, dataframe.String)}, nil, "unsupported"},
		{"datetime", []dataframe.Column{column([]int64{1}, dataframe.DateTime)}, nil, "unsupported"},
		{"category", []dataframe.Column{&dataframe.CategoryColumn{TypedColumn: *column([]uint32{0}, dataframe.Category), Dictionary: []string{"1"}}}, nil, "unsupported"},
		{"mismatched float type", []dataframe.Column{column([]string{"1"}, dataframe.Float64)}, nil, "unsupported"},
		{"mismatched integer type", []dataframe.Column{column([]string{"1"}, dataframe.Int64)}, nil, "int64 values"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stat.Mean(tt.columns, tt.opts...)
			if err == nil || !strings.Contains(err.Error(), tt.message) || got != 0 {
				t.Fatalf("got %g, %v; want zero and error containing %q", got, err, tt.message)
			}
		})
	}
}
