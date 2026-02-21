package rds

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
)

var ordinalRegex = regexp.MustCompile("\\?{1}")

// NewMySQL dialect from our configuration
func NewMySQL(config *Config) Dialect {
	return &DialectMySQL{parseTime: config.ParseTime}
}

// DialectMySQL for version 5.7
type DialectMySQL struct {
	parseTime bool
}

// MigrateQuery converts a mysql queries into an RDS stateement.
func (d *DialectMySQL) MigrateQuery(query string, args []driver.NamedValue) (*rdsdata.ExecuteStatementInput, error) {
	// Make sure we're not mixing and matching.
	ordinal := false
	named := false
	for _, arg := range args {
		if arg.Name != "" {
			named = true
		}
		if arg.Ordinal > 0 {
			ordinal = true
		}
		if named && ordinal {
			return nil, ErrNoMixedParams
		}
	}

	// If we're ordinal, convert to named
	if ordinal {
		namedArgs := make([]driver.NamedValue, len(args))
		for i, v := range args {
			namedArgs[i] = driver.NamedValue{
				Name:  strconv.Itoa(v.Ordinal),
				Value: v.Value,
			}
		}

		idx := 0
		query = ordinalRegex.ReplaceAllStringFunc(query, func(s string) string {
			idx++ // ordinal regex are one-indexed
			return fmt.Sprintf(":%d", idx)
		})

		params, err := ConvertNamedValues(namedArgs)
		return &rdsdata.ExecuteStatementInput{
			Parameters: params,
			Sql:        aws.String(query),
		}, err
	}

	params, err := ConvertNamedValues(args)
	return &rdsdata.ExecuteStatementInput{
		Parameters: params,
		Sql:        aws.String(query),
	}, err
}

// GetFieldConverter knows how to parse column results.
func (d *DialectMySQL) GetFieldConverter(columnType string) FieldConverter {
	switch columnType {
	case "TINYINT UNSIGNED":
		fallthrough
	case "SMALLINT UNSIGNED":
		fallthrough
	case "MEDIUMINT UNSIGNED":
		fallthrough
	case "INT UNSIGNED":
		fallthrough
	case "BIGINT UNSIGNED":
		return func(field types.Field) (interface{}, error) {
			longValue := field.(*types.FieldMemberLongValue).Value
			if longValue < 0 {
				return nil, fmt.Errorf("cannot convert negative value %d to uint64", longValue)
			}
			return uint64(longValue), nil
		}
	case "DECIMAL":
		return func(field types.Field) (interface{}, error) {
			return strconv.ParseFloat(field.(*types.FieldMemberStringValue).Value, 64)
		}
	case "BIT":
		// Bit values appear to be returned as boolean values
		return func(field types.Field) (interface{}, error) {
			if field.(*types.FieldMemberBooleanValue).Value {
				return 1, nil
			}
			return 0, nil
		}
	case "DATE":
		return func(field types.Field) (interface{}, error) {
			dateStringVal := field.(*types.FieldMemberStringValue).Value
			if d.parseTime {
				return time.Parse("2006-01-02", dateStringVal)
			}
			return dateStringVal, nil
		}
	case "TIME":
		return func(field types.Field) (interface{}, error) {
			timeStringVal := field.(*types.FieldMemberStringValue).Value
			if d.parseTime {
				return time.Parse("15:04:05", timeStringVal)
			}
			return timeStringVal, nil
		}
	case "DATETIME":
		return func(field types.Field) (interface{}, error) {
			dateTimeStringVal := field.(*types.FieldMemberStringValue).Value
			if d.parseTime {
				return time.Parse("2006-01-02 15:04:05", dateTimeStringVal)
			}
			return dateTimeStringVal, nil
		}
	case "TIMESTAMP":
		return func(field types.Field) (interface{}, error) {
			timestampStringVal := field.(*types.FieldMemberStringValue).Value
			if d.parseTime {
				return time.Parse("2006-01-02 15:04:05", timestampStringVal)
			}
			return timestampStringVal, nil
		}
	case "YEAR":
		// RDS sends a full date string. MySQL only returns the year.
		return func(field types.Field) (interface{}, error) {
			t, err := time.Parse("2006-01-02", field.(*types.FieldMemberStringValue).Value)
			if err != nil {
				return nil, err
			}
			if d.parseTime {
				return t, nil
			}
			return strconv.Itoa(t.Year()), nil
		}
	}

	return ConvertDefaults()
}

// IsIsolationLevelSupported for mysql?
func (d *DialectMySQL) IsIsolationLevelSupported(level driver.IsolationLevel) bool {
	// SupportedIsolationLevels for the dialect
	var SupportedIsolationLevels = map[driver.IsolationLevel]bool{
		driver.IsolationLevel(sql.LevelDefault):         true,
		driver.IsolationLevel(sql.LevelRepeatableRead):  true,
		driver.IsolationLevel(sql.LevelReadCommitted):   true,
		driver.IsolationLevel(sql.LevelReadUncommitted): true,
		driver.IsolationLevel(sql.LevelSerializable):    true,
	}
	_, ok := SupportedIsolationLevels[level]
	return ok
}

// GetTransactionSetupQuery returns the query to set up the transaction.
func (d *DialectMySQL) GetTransactionSetupQuery(opts driver.TxOptions) string {
	if sql.IsolationLevel(opts.Isolation) == sql.LevelDefault && !opts.ReadOnly {
		return ""
	}
	var clause []string
	if sql.IsolationLevel(opts.Isolation) != sql.LevelDefault {
		clause = append(clause, fmt.Sprintf("ISOLATION LEVEL %s", sql.IsolationLevel(opts.Isolation).String()))
	}
	if opts.ReadOnly {
		clause = append(clause, "READ ONLY")
	} else {
		clause = append(clause, "READ WRITE")
	}
	return fmt.Sprintf("SET TRANSACTION %s", strings.Join(clause, ", "))
}
