package stat

import (
	"fmt"
	"math"
	"reflect"
	"slices"

	"github.com/axent-pl/dataframe"
	gonumstat "gonum.org/v1/gonum/stat"
)

// MeanOption configures which observations Mean includes.
type MeanOption func(*meanOptions) error

type meanOptions struct {
	skipNulls     bool
	skipNonFinite bool
}

// WithSkipNulls controls whether Mean ignores null observations (default true).
// If false, encountering a null returns an error.
func WithSkipNulls(skip bool) MeanOption {
	return func(o *meanOptions) error {
		o.skipNulls = skip
		return nil
	}
}

// WithSkipNonFinite controls whether Mean ignores NaN and infinities (default
// false). If false, encountering a non-null, non-finite value returns an error.
func WithSkipNonFinite(skip bool) MeanOption {
	return func(o *meanOptions) error {
		o.skipNonFinite = skip
		return nil
	}
}

// Mean returns the arithmetic mean of all included values across columns.
// Every observation has equal weight, even when columns have different lengths.
// It accepts Float64 and Int64 columns; integers are converted to float64 and
// may lose precision beyond 53 bits. The input columns are not modified.
//
// By default, nulls are skipped and NaN or infinities cause an error. Nil or
// unsupported columns, nil options, and inputs with no included values also
// return errors. Empty columns may be mixed with nonempty columns.
func Mean(columns []dataframe.Column, opts ...MeanOption) (float64, error) {
	o := meanOptions{skipNulls: true}
	for _, opt := range opts {
		if opt == nil {
			return 0, fmt.Errorf("stat: nil mean option")
		}
		if err := opt(&o); err != nil {
			return 0, err
		}
	}

	var values []float64
	for i, column := range columns {
		if column == nil || (reflect.ValueOf(column).Kind() == reflect.Ptr && reflect.ValueOf(column).IsNil()) {
			return 0, fmt.Errorf("stat: nil mean column at index %d", i)
		}
		var err error
		switch column.Type() {
		case dataframe.Float64:
			if c, ok := column.(valueColumn[float64]); ok {
				values, err = appendMeanValues(values, c, i, o)
				break
			}
			fallthrough
		default:
			return 0, fmt.Errorf("stat: unsupported mean column %T with type %s at index %d", column, column.Type(), i)
		case dataframe.Int64:
			c, ok := column.(valueColumn[int64])
			if !ok {
				return 0, fmt.Errorf("stat: mean column at index %d does not provide int64 values", i)
			}
			values, err = appendMeanValues(values, c, i, o)
		}
		if err != nil {
			return 0, err
		}
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("stat: mean requires at least one included value")
	}

	mean := gonumstat.Mean(values, nil)
	if !finite(mean) {
		// A sum of finite values can overflow even though their mean is finite.
		// Scale our private buffer and retry without changing input columns.
		var scale float64
		for _, x := range values {
			scale = math.Max(scale, math.Abs(x))
		}
		for i := range values {
			values[i] /= scale
		}
		mean = gonumstat.Mean(values, nil) * scale
	}
	return mean, nil
}

func appendMeanValues[T interface{ ~float64 | ~int64 }](values []float64, column valueColumn[T], index int, o meanOptions) ([]float64, error) {
	values = slices.Grow(values, column.Len())
	for i := 0; i < column.Len(); i++ {
		if column.IsNull(i) {
			if !o.skipNulls {
				return nil, fmt.Errorf("stat: null mean value at column %d, row %d", index, i)
			}
			continue
		}
		x := float64(column.Get(i))
		if !finite(x) {
			if !o.skipNonFinite {
				return nil, fmt.Errorf("stat: non-finite mean value at column %d, row %d", index, i)
			}
			continue
		}
		values = append(values, x)
	}
	return values, nil
}
