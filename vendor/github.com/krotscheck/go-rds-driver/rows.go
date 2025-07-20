package rds

import (
	"database/sql/driver"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
	"io"
)

// Explicit compile time checks.
var _ driver.Rows = (*Rows)(nil)
var _ driver.RowsNextResultSet = (*Rows)(nil) // explicit compile time type check
//var _ driver.RowsColumnTypeScanType = (*Rows)(nil)         // explicit compile time type check
//var _ driver.RowsColumnTypeDatabaseTypeName = (*Rows)(nil) // explicit compile time type check
//var _ driver.RowsColumnTypeLength = (*Rows)(nil)           // explicit compile time type check
//var _ driver.RowsColumnTypeNullable = (*Rows)(nil)         // explicit compile time type check
//var _ driver.RowsColumnTypePrecisionScale = (*Rows)(nil)   // explicit compile time type check

// NewRows instance for the provided statement output
func NewRows(dialect Dialect, results []*rdsdata.ExecuteStatementOutput) driver.Rows {
	rows := &Rows{
		results:        results,
		recordPosition: 0,
		resultPosition: 0,
		dialect:        dialect,
	}
	rows.setResultIndex(0)
	return rows
}

// Rows implementation for the RDS Driver
type Rows struct {
	dialect        Dialect
	resultPosition int
	results        []*rdsdata.ExecuteStatementOutput

	columnNames    []string
	converters     []FieldConverter
	recordPosition int
}

// HasNextResultSet returns true if there's another result set.
func (r *Rows) HasNextResultSet() bool {
	return r.resultPosition+1 < len(r.results)
}

// NextResultSet moves the result to the next result set.
func (r *Rows) NextResultSet() error {
	if r.HasNextResultSet() {
		r.setResultIndex(r.resultPosition + 1)
		return nil
	}
	return io.EOF
}

func (r *Rows) setResultIndex(i int) {
	r.resultPosition = i
	r.recordPosition = 0
	curr := r.results[r.resultPosition]

	r.converters = make([]FieldConverter, len(curr.ColumnMetadata))
	r.columnNames = make([]string, len(curr.ColumnMetadata))
	for i, col := range curr.ColumnMetadata {
		r.converters[i] = r.dialect.GetFieldConverter(*col.TypeName)
		r.columnNames[i] = *col.Label
	}
}

// Columns returns the column names in order
func (r *Rows) Columns() []string {
	return r.columnNames
}

// Close the result set
func (r *Rows) Close() error {
	// The API is stateless, so there's no connection to close
	return nil
}

// Next row in the result set
func (r *Rows) Next(dest []driver.Value) error {
	curr := r.results[r.resultPosition]
	if r.recordPosition == len(curr.Records) {
		return io.EOF
	}

	row := curr.Records[r.recordPosition]
	r.recordPosition++

	for i, field := range row {
		switch field.(type) {
		case *types.FieldMemberIsNull:
			dest[i] = nil
			continue
		default:
			break
		}

		converter := r.converters[i]
		coerced, err := converter(field)

		if err != nil {
			fmt.Printf("Metadata for failed column: %#v", curr.ColumnMetadata[i])
			return fmt.Errorf("convertValue(col=%d): %v", i, err)
		}

		dest[i] = coerced
	}

	return nil
}
