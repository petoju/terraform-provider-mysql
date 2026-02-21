package rds

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
)

var _ driver.Conn = (*Connection)(nil)               // explicit compile time type check
var _ driver.ConnPrepareContext = (*Connection)(nil) // explicit compile time type check
var _ driver.ConnBeginTx = (*Connection)(nil)        // explicit compile time type check
var _ driver.ExecerContext = (*Connection)(nil)      // explicit compile time type check
var _ driver.Pinger = (*Connection)(nil)             // explicit compile time type check
var _ driver.QueryerContext = (*Connection)(nil)     // explicit compile time type check
var _ driver.SessionResetter = (*Connection)(nil)    // explicit compile time type check
var _ driver.Validator = (*Connection)(nil)          // explicit compile time type check
// var _ driver.NamedValueChecker = (*Connection)(nil)  // explicit compile time type check

// NewConnection that can make transaction and statement requests against RDS
func NewConnection(ctx context.Context, rds AWSClientInterface, conf *Config, dialect Dialect) driver.Conn {
	return &Connection{
		ctx:         ctx,
		rds:         rds,
		resourceARN: conf.ResourceArn,
		secretARN:   conf.SecretArn,
		database:    conf.Database,
		splitMulti:  conf.SplitMulti,
		closed:      false,
		dialect:     dialect,
	}
}

// Connection to RDS's Aurora Serverless Data API
type Connection struct {
	ctx         context.Context
	rds         AWSClientInterface
	resourceARN string
	secretARN   string
	database    string
	splitMulti  bool
	tx          *Tx // The current transaction, if set
	closed      bool
	dialect     Dialect
}

// Ping the database
func (r *Connection) Ping(ctx context.Context) (err error) {
	_, err = r.rds.ExecuteStatement(ctx, &rdsdata.ExecuteStatementInput{
		ResourceArn: &r.resourceARN,
		Database:    &r.database,
		SecretArn:   &r.secretARN,
		Sql:         aws.String("/* ping */ SELECT 1"), // This works for all databases, I think.
		Parameters:  []types.SqlParameter{},
	})
	return
}

// Prepare returns a prepared statement, bound to this connection.
func (r *Connection) Prepare(query string) (driver.Stmt, error) {
	return r.PrepareContext(context.Background(), query)
}

// PrepareContext returns a prepared statement, bound to this connection.
func (r *Connection) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	queries := []string{query}
	if r.splitMulti {
		queries = strings.Split(query, ";")
		queries = removeEmptyQueries(queries)
	}
	return NewStatement(ctx, r, queries), nil
}

// Close the connection
func (r *Connection) Close() error {
	if r.closed {
		return ErrClosed
	}
	if r.tx != nil {
		if err := r.tx.Rollback(); err != nil {
			return err
		}
	}
	r.rds = nil
	r.closed = true
	return nil
}

// Begin starts and returns a new transaction.
func (r *Connection) Begin() (driver.Tx, error) {
	return r.BeginTx(r.ctx, driver.TxOptions{
		Isolation: driver.IsolationLevel(sql.LevelDefault),
		ReadOnly:  false,
	})
}

// BeginTx starts and returns a new transaction.
func (r *Connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if !r.dialect.IsIsolationLevelSupported(opts.Isolation) {
		return nil, fmt.Errorf("isolation level %d not supported", opts.Isolation)
	}

	output, err := r.rds.BeginTransaction(ctx, &rdsdata.BeginTransactionInput{
		Database:    aws.String(r.database),
		ResourceArn: aws.String(r.resourceARN),
		SecretArn:   aws.String(r.secretARN),
	})
	if err != nil {
		return nil, err
	}
	r.tx = &Tx{
		Done:          false,
		TransactionID: output.TransactionId,
		conn:          r,
	}

	query := r.dialect.GetTransactionSetupQuery(opts)
	if query != "" {
		if _, err := r.ExecContext(ctx, query, nil); err != nil {
			defer func() {
				_ = r.tx.Rollback()
			}()
			return nil, err
		}
	}

	return r.tx, nil
}

// ResetSession is called prior to executing a queries on the connection
// if the connection has been used before. If the driver returns ErrBadConn
// the connection is discarded.
func (r *Connection) ResetSession(_ context.Context) error {
	if r.tx != nil {
		if err := r.tx.Rollback(); err != nil {
			return err
		}
		return driver.ErrBadConn
	}
	return nil
}

// IsValid is called prior to placing the connection into the connection pool. The connection will be discarded if false is returned.
func (r *Connection) IsValid() bool {
	if r.closed {
		return false
	}
	if r.database == "" {
		return false
	}
	if r.resourceARN == "" {
		return false
	}
	if r.secretARN == "" {
		return false
	}
	return true
}

// QueryContext executes a statement that would return some kind of result.
func (r *Connection) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	stmt, err := r.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	if st, ok := stmt.(driver.StmtQueryContext); ok {
		return st.QueryContext(ctx, args)
	}
	return nil, fmt.Errorf("invalid statement")
}

// ExecContext executes a queries that would normally not return a result.
func (r *Connection) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	stmt, err := r.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	if st, ok := stmt.(driver.StmtExecContext); ok {
		return st.ExecContext(ctx, args)
	}
	return nil, fmt.Errorf("invalid statement")
}

func removeEmptyQueries(s []string) []string {
	var r []string
	for _, str := range s {
		str = strings.TrimSpace(str)
		if str != "" {
			r = append(r, str)
		}
	}
	return r
}
