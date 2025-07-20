package rds

import (
	"context"
	"database/sql/driver"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
)

var _ driver.Stmt = (*Statement)(nil)             // explicit compile time type check
var _ driver.StmtExecContext = (*Statement)(nil)  // explicit compile time type check
var _ driver.StmtQueryContext = (*Statement)(nil) // explicit compile time type check
//var _ driver.NamedValueChecker = (*Statement)(nil) // explicit compile time type check

// NewStatement for the provided connection
func NewStatement(_ context.Context, connection *Connection, sql []string) *Statement {
	return &Statement{
		conn:    connection,
		queries: sql,
	}
}

// Statement encapsulates a single RDS queries statement
type Statement struct {
	conn    *Connection
	queries []string
}

// Close closes the statement.
func (s *Statement) Close() error {
	if s.conn == nil {
		return ErrClosed
	}
	s.conn = nil
	return nil
}

// NumInput returns the number of placeholder parameters.
func (s *Statement) NumInput() int {
	return -1
}

// Exec executes a queries that doesn't return rows, such as an INSERT or UPDATE.
func (s *Statement) Exec(values []driver.Value) (driver.Result, error) {
	args := s.ConvertOrdinal(values)
	return s.ExecContext(context.Background(), args)
}

// ExecContext executes a queries that doesn't return rows, such as an INSERT or UPDATE.
func (s *Statement) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	var output []*rdsdata.ExecuteStatementOutput
	for _, query := range s.queries {
		out, err := s.executeStatement(ctx, query, args)
		if err != nil {
			return nil, err
		}
		output = append(output, out)
	}
	return NewResult(output), nil
}

// Query executes a queries that may return rows, such as a SELECT.
func (s *Statement) Query(values []driver.Value) (driver.Rows, error) {
	// We're trying to execute this as an ordinal queries, so convert it.
	args := s.ConvertOrdinal(values)
	return s.QueryContext(context.Background(), args)
}

// QueryContext executes a queries that may return rows, such as a SELECT.
func (s *Statement) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	var output []*rdsdata.ExecuteStatementOutput
	for _, query := range s.queries {
		out, err := s.executeStatement(ctx, query, args)
		if err != nil {
			return nil, err
		}
		output = append(output, out)
	}
	return NewRows(s.conn.dialect, output), nil
}

// ConvertOrdinal converts a list of Values to Ordinal NamedValues
func (s *Statement) ConvertOrdinal(values []driver.Value) []driver.NamedValue {
	// Start with the MySQL separator as a default
	namedValues := make([]driver.NamedValue, len(values))
	for i, v := range values {
		namedValues[i] = driver.NamedValue{
			Name:    "",
			Ordinal: i + 1,
			Value:   v,
		}
	}
	return namedValues
}

func (s *Statement) executeStatement(ctx context.Context, query string, values []driver.NamedValue) (*rdsdata.ExecuteStatementOutput, error) {
	input, err := s.conn.dialect.MigrateQuery(query, values)

	if err != nil {
		return nil, err
	}

	if s.conn.tx != nil {
		input.TransactionId = s.conn.tx.TransactionID
	}

	input.IncludeResultMetadata = true
	input.ResourceArn = aws.String(s.conn.resourceARN)
	input.SecretArn = aws.String(s.conn.secretARN)
	input.Database = aws.String(s.conn.database)
	return s.conn.rds.ExecuteStatement(ctx, input)
}
