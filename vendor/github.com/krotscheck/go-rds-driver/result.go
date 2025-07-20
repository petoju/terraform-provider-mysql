package rds

import (
	"database/sql/driver"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
)

var _ driver.Result = (*Result)(nil) // explicit compile time type check

// NewResult for the executed statement
func NewResult(results []*rdsdata.ExecuteStatementOutput) driver.Result {
	// Calculate the total number of affected rows
	var rowsAffected int64 = 0
	var lastInsertID int64 = 0
	for _, r := range results {
		// Calculate the number of affected rows
		rowsAffected = r.NumberOfRecordsUpdated + rowsAffected

		// Calculate the last inserted ID
		if l := len(r.GeneratedFields); l == 0 {
			continue
		} else if l != 1 {
			continue
		}

		field := r.GeneratedFields[0]
		switch fv := field.(type) {
		case *types.FieldMemberLongValue:
			lastInsertID = fv.Value
		default:
			continue
		}
	}

	return &Result{
		rowsAffected: rowsAffected,
		lastInsertID: lastInsertID,
	}
}

// Result from a queries
type Result struct {
	rowsAffected int64
	lastInsertID int64
}

// LastInsertId from the executed statements.
func (r *Result) LastInsertId() (int64, error) {
	return r.lastInsertID, nil
}

// RowsAffected count
func (r *Result) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}
