package dataframe

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type CSVOptions struct {
	HasHeader          bool
	SampleRows         int
	NullValues         []string
	DateFormats        []string
	Schema             *Schema
	DictionaryEncoding bool
	CategoryThreshold  float64
}

func DefaultCSVOptions() CSVOptions {
	return CSVOptions{
		HasHeader:          true,
		SampleRows:         1000,
		NullValues:         []string{"", "NA", "N/A", "null", "NULL"},
		DateFormats:        []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"},
		DictionaryEncoding: true,
		CategoryThreshold:  0.2,
	}
}

func LoadCSV(r io.Reader, opts CSVOptions) (*DataFrame, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty csv")
	}
	header := []string{}
	start := 0
	if opts.HasHeader {
		header = records[0]
		start = 1
	} else {
		for i := range records[0] {
			header = append(header, fmt.Sprintf("column_%d", i+1))
		}
	}
	if len(header) == 0 {
		return nil, fmt.Errorf("no columns")
	}
	for i := start; i < len(records); i++ {
		if len(records[i]) != len(header) {
			return nil, fmt.Errorf("row %d: expected %d fields, got %d", i+1, len(header), len(records[i]))
		}
	}
	nulls := make(map[string]bool)
	for _, v := range opts.NullValues {
		nulls[v] = true
	}
	if opts.NullValues == nil {
		nulls[""] = true
	}
	layouts := opts.DateFormats
	if len(layouts) == 0 {
		layouts = DefaultCSVOptions().DateFormats
	}
	fields := make([]ColumnSpec, len(header))
	if opts.Schema != nil {
		if len(opts.Schema.Columns) != len(header) {
			return nil, fmt.Errorf("schema column count mismatch")
		}
		copy(fields, opts.Schema.Columns)
		for i := range fields {
			if fields[i].Name != header[i] {
				return nil, fmt.Errorf("schema/header mismatch at %d", i)
			}
		}
	} else {
		limit := len(records)
		if opts.SampleRows > 0 && start+opts.SampleRows < limit {
			limit = start + opts.SampleRows
		}
		for j, name := range header {
			typ := Int64
			nonNull := 0
			unique := map[string]struct{}{}
			nullable := false
			for i := start; i < limit; i++ {
				v := records[i][j]
				if nulls[v] {
					nullable = true
					continue
				}
				nonNull++
				unique[v] = struct{}{}
				typ = promote(typ, v, layouts)
			}
			if nonNull == 0 {
				typ = String
			}
			if typ == String && opts.DictionaryEncoding && nonNull > 0 && float64(len(unique))/float64(nonNull) <= opts.CategoryThreshold {
				typ = Category
			}
			fields[j] = ColumnSpec{Name: name, Type: typ, Nullable: nullable}
		}
	}
	columns := make([]Column, len(fields))
	n := len(records) - start
	for j, f := range fields {
		switch f.Type {
		case Float64:
			data := make([]float64, n)
			b := newBitmap(n)
			for i := 0; i < n; i++ {
				v := records[start+i][j]
				if nulls[v] {
					b.Set(i)
					continue
				}
				x, e := strconv.ParseFloat(v, 64)
				if e != nil {
					return nil, cellErr(i, j, e)
				}
				data[i] = x
			}
			columns[j] = &TypedColumn[float64]{Data: data, Nulls: b, DType: Float64}
		case Int64, DateTime:
			data := make([]int64, n)
			b := newBitmap(n)
			for i := 0; i < n; i++ {
				v := records[start+i][j]
				if nulls[v] {
					b.Set(i)
					continue
				}
				var x int64
				var e error
				if f.Type == Int64 {
					x, e = strconv.ParseInt(v, 10, 64)
				} else {
					x, e = parseDate(v, layouts)
				}
				if e != nil {
					return nil, cellErr(i, j, e)
				}
				data[i] = x
			}
			columns[j] = &TypedColumn[int64]{Data: data, Nulls: b, DType: f.Type}
		case Bool:
			data := make([]bool, n)
			b := newBitmap(n)
			for i := 0; i < n; i++ {
				v := records[start+i][j]
				if nulls[v] {
					b.Set(i)
					continue
				}
				x, e := parseBool(v)
				if e != nil {
					return nil, cellErr(i, j, e)
				}
				data[i] = x
			}
			columns[j] = &TypedColumn[bool]{Data: data, Nulls: b, DType: Bool}
		case String:
			data := make([]string, n)
			b := newBitmap(n)
			for i := 0; i < n; i++ {
				v := records[start+i][j]
				if nulls[v] {
					b.Set(i)
				} else {
					data[i] = v
				}
			}
			columns[j] = &TypedColumn[string]{Data: data, Nulls: b, DType: String}
		case Category:
			data := make([]uint32, n)
			b := newBitmap(n)
			dict := []string{}
			ids := map[string]uint32{}
			for i := 0; i < n; i++ {
				v := records[start+i][j]
				if nulls[v] {
					b.Set(i)
					continue
				}
				id, ok := ids[v]
				if !ok {
					id = uint32(len(dict))
					ids[v] = id
					dict = append(dict, v)
				}
				data[i] = id
			}
			columns[j] = &CategoryColumn{TypedColumn: TypedColumn[uint32]{Data: data, Nulls: b, DType: Category}, Dictionary: dict}
		default:
			return nil, fmt.Errorf("unsupported type %s", f.Type)
		}
	}
	return NewDataFrame(fields, columns)
}
func cellErr(i, j int, e error) error { return fmt.Errorf("data row %d, column %d: %w", i+1, j+1, e) }
func parseBool(v string) (bool, error) {
	switch strings.ToLower(v) {
	case "true", "t", "yes", "y":
		return true, nil
	case "false", "f", "no", "n":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q", v)
}
func parseDate(v string, layouts []string) (int64, error) {
	for _, l := range layouts {
		if t, e := time.Parse(l, v); e == nil {
			return t.UnixMicro(), nil
		}
	}
	return 0, fmt.Errorf("invalid date %q", v)
}
func promote(t DataType, v string, layouts []string) DataType {
	if t == String {
		return String
	}
	if t == Int64 {
		if _, e := strconv.ParseInt(v, 10, 64); e == nil {
			return Int64
		}
		t = Float64
	}
	if t == Float64 {
		if _, e := strconv.ParseFloat(v, 64); e == nil {
			return Float64
		}
		t = Bool
	}
	if t == Bool {
		if _, e := parseBool(v); e == nil {
			return Bool
		}
		t = DateTime
	}
	if t == DateTime {
		if _, e := parseDate(v, layouts); e == nil {
			return DateTime
		}
	}
	return String
}
