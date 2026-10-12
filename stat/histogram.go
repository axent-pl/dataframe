// Package stat provides statistical transformations of dataframe columns.
package stat

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"

	"github.com/axent-pl/dataframe"
	"gonum.org/v1/gonum/floats"
)

// HistogramOption configures histogram binning.
type HistogramOption func(*histogramOptions) error

type histogramOptions struct {
	bins     int
	binsSet  bool
	maxBins  int
	rangeSet bool
	min, max float64
	edges    []float64
}

// WithBins sets the number of equal-width numeric bins (default 10).
// Constant data produces one bin; indistinguishable floating-point edges are
// collapsed. WithMaxBins may further reduce this number.
func WithBins(n int) HistogramOption {
	return func(o *histogramOptions) error {
		if err := validBinCount(n); err != nil {
			return err
		}
		o.bins, o.binsSet = n, true
		return nil
	}
}

// WithMaxBins caps the number of bins. Equal-width numeric bins are reduced to
// this limit. Explicit edges or discrete labels exceeding it cause an error.
func WithMaxBins(n int) HistogramOption {
	return func(o *histogramOptions) error {
		if err := validBinCount(n); err != nil {
			return err
		}
		o.maxBins = n
		return nil
	}
}

// WithRange sets the inclusive numeric range instead of inferring it from the
// data. Bounds must be finite and min < max. Values outside it become null.
func WithRange(min, max float64) HistogramOption {
	return func(o *histogramOptions) error {
		if !finite(min) || !finite(max) || min >= max {
			return fmt.Errorf("stat: histogram range must have finite bounds with min < max")
		}
		o.min, o.max, o.rangeSet = min, max, true
		return nil
	}
}

// WithBinEdges supplies finite, strictly increasing numeric bin boundaries.
// It cannot be combined with WithBins or WithRange. The edges are copied.
func WithBinEdges(edges ...float64) HistogramOption {
	edges = slices.Clone(edges)
	return func(o *histogramOptions) error {
		if len(edges) < 2 {
			return fmt.Errorf("stat: histogram needs at least two bin edges")
		}
		if err := validBinCount(len(edges) - 1); err != nil {
			return err
		}
		for i, edge := range edges {
			if !finite(edge) || (i > 0 && edge <= edges[i-1]) {
				return fmt.Errorf("stat: histogram bin edges must be finite and strictly increasing")
			}
		}
		o.edges = edges
		return nil
	}
}

// Histogram returns one bin ID per input row, with bin labels in Dictionary.
// The input is not modified and the result owns its data and null bitmap.
//
// Float64 and Int64 columns use equal-width bins over their finite values by
// default. Numeric calculations convert values to float64. Labels describe
// intervals [lower, upper), except the final interval includes its upper bound.
// Nulls, NaN, infinities, and values outside the chosen range become null.
// Empty or entirely invalid numeric data has no bins unless a range or edges
// are supplied.
//
// Bool, String, and Category columns use distinct observed labels as bins in
// first-appearance order. Unused dictionary entries are omitted. Only
// WithMaxBins applies to these discrete columns; numeric options cause an error.
// Unsupported column types and invalid options also return errors.
func Histogram(column dataframe.Column, opts ...HistogramOption) (*dataframe.CategoryColumn, error) {
	if column == nil || (reflect.ValueOf(column).Kind() == reflect.Ptr && reflect.ValueOf(column).IsNil()) {
		return nil, fmt.Errorf("stat: nil histogram column")
	}
	o := histogramOptions{bins: 10}
	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("stat: nil histogram option")
		}
		if err := opt(&o); err != nil {
			return nil, err
		}
	}
	if o.edges != nil {
		if o.rangeSet || o.binsSet {
			return nil, fmt.Errorf("stat: bin edges cannot be combined with a range or bin count")
		}
		if o.maxBins > 0 && len(o.edges)-1 > o.maxBins {
			return nil, fmt.Errorf("stat: bin edges exceed maximum bins (%d)", o.maxBins)
		}
	}
	if o.maxBins > 0 && o.bins > o.maxBins {
		o.bins = o.maxBins
	}
	if column.Type() != dataframe.Float64 && column.Type() != dataframe.Int64 &&
		(o.rangeSet || o.binsSet || o.edges != nil) {
		return nil, fmt.Errorf("stat: numeric binning options require a numeric column")
	}

	switch column.Type() {
	case dataframe.Float64:
		if c, ok := column.(valueColumn[float64]); ok {
			return numericHistogram(c, o), nil
		}
	case dataframe.Int64:
		if c, ok := column.(valueColumn[int64]); ok {
			return numericHistogram(c, o), nil
		}
	case dataframe.Bool:
		if c, ok := column.(valueColumn[bool]); ok {
			return discreteHistogram(c, func(i int) (string, error) {
				return strconv.FormatBool(c.Get(i)), nil
			}, o.maxBins)
		}
	case dataframe.String:
		if c, ok := column.(valueColumn[string]); ok {
			return discreteHistogram(c, func(i int) (string, error) {
				return c.Get(i), nil
			}, o.maxBins)
		}
	case dataframe.Category:
		if c, ok := column.(*dataframe.CategoryColumn); ok {
			return discreteHistogram(c, func(i int) (string, error) {
				id := c.Get(i)
				if uint64(id) >= uint64(len(c.Dictionary)) {
					return "", fmt.Errorf("stat: invalid category ID %d at row %d", id, i)
				}
				return c.Dictionary[id], nil
			}, o.maxBins)
		}
	}
	return nil, fmt.Errorf("stat: unsupported histogram column %T with type %s", column, column.Type())
}

type valueColumn[T dataframe.Value] interface {
	dataframe.Column
	Get(int) T
}

func numericHistogram[T interface{ ~float64 | ~int64 }](column valueColumn[T], o histogramOptions) *dataframe.CategoryColumn {
	result := newResult(column.Len())
	edges := o.edges
	if edges == nil {
		lo, hi := o.min, o.max
		if !o.rangeSet {
			lo, hi = math.Inf(1), math.Inf(-1)
			for i := 0; i < column.Len(); i++ {
				if column.IsNull(i) {
					continue
				}
				x := float64(column.Get(i))
				if finite(x) {
					lo, hi = math.Min(lo, x), math.Max(hi, x)
				}
			}
		}
		switch {
		case lo > hi: // No finite, non-null observations.
			for i := range result.Data {
				result.Nulls.Set(i)
			}
			return result
		case lo == hi:
			edges = []float64{lo, hi}
		default:
			edges = make([]float64, o.bins+1)
			if finite(hi - lo) {
				floats.Span(edges, lo, hi)
			} else {
				// Interpolate without overflowing the width of a finite range.
				floats.Span(edges, 0, 1)
				for i, fraction := range edges {
					edges[i] = (1-fraction)*lo + fraction*hi
				}
			}
			edges[0], edges[len(edges)-1] = lo, hi
			edges = slices.Compact(edges)
		}
	}

	result.Dictionary = make([]string, len(edges)-1)
	for i := range result.Dictionary {
		end := ")"
		if i == len(result.Dictionary)-1 {
			end = "]"
		}
		result.Dictionary[i] = "[" + strconv.FormatFloat(edges[i], 'g', -1, 64) + ", " +
			strconv.FormatFloat(edges[i+1], 'g', -1, 64) + end
	}
	lo, hi := edges[0], edges[len(edges)-1]
	for i := range result.Data {
		if column.IsNull(i) {
			result.Nulls.Set(i)
			continue
		}
		x := float64(column.Get(i))
		if !finite(x) || x < lo || x > hi {
			result.Nulls.Set(i)
			continue
		}
		// Search only upper boundaries. The last bin also includes hi.
		bin := sort.Search(len(edges)-2, func(j int) bool { return x < edges[j+1] })
		result.Data[i] = uint32(bin)
	}
	return result
}

func discreteHistogram(column dataframe.Column, label func(int) (string, error), maxBins int) (*dataframe.CategoryColumn, error) {
	result := newResult(column.Len())
	ids := make(map[string]uint32)
	for i := range result.Data {
		if column.IsNull(i) {
			result.Nulls.Set(i)
			continue
		}
		value, err := label(i)
		if err != nil {
			return nil, err
		}
		id, ok := ids[value]
		if !ok {
			if maxBins > 0 && len(ids) >= maxBins {
				return nil, fmt.Errorf("stat: distinct labels exceed maximum bins (%d)", maxBins)
			}
			if uint64(len(ids)) >= uint64(math.MaxUint32) {
				return nil, fmt.Errorf("stat: too many histogram bins")
			}
			id = uint32(len(result.Dictionary))
			ids[value] = id
			result.Dictionary = append(result.Dictionary, value)
		}
		result.Data[i] = id
	}
	return result, nil
}

func newResult(n int) *dataframe.CategoryColumn {
	return &dataframe.CategoryColumn{TypedColumn: dataframe.TypedColumn[uint32]{
		Data: make([]uint32, n), Nulls: dataframe.NewBitmap(n), DType: dataframe.Category,
	}}
}

func validBinCount(n int) error {
	if n <= 0 || uint64(n) > uint64(math.MaxUint32) || n == int(^uint(0)>>1) {
		return fmt.Errorf("stat: histogram bin count must be positive and fit category IDs")
	}
	return nil
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }
