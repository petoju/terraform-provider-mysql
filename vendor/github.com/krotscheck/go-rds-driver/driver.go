package rds

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
)

// DRIVERNAME is used when configuring your dialector
const DRIVERNAME = "rds"

var _ driver.Driver = (*Driver)(nil)        // explicit compile time type check
var _ driver.DriverContext = (*Driver)(nil) // explicit compile time type check

// NewDriver creates a new driver instance for RDS
func NewDriver() *Driver {
	return &Driver{}
}

// Driver implements the driver.Driver interface for RDS
type Driver struct{}

// Open returns a new connection to the database.
func (r *Driver) Open(name string) (driver.Conn, error) {
	connector, err := r.OpenConnector(name)
	if err != nil {
		return nil, err
	}
	return connector.Connect(context.Background())
}

// OpenConnector must parse the name in the same format that Driver.Open parses the name parameter.
func (r *Driver) OpenConnector(dsn string) (driver.Connector, error) {
	conf, err := NewConfigFromDSN(dsn)
	if err != nil {
		return nil, err
	}

	awsConfig, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(conf.AWSRegion))

	if err != nil {
		return nil, err
	}

	client := rdsdata.NewFromConfig(awsConfig)

	return NewConnector(r, client, conf), nil
}

func init() {
	sql.Register(DRIVERNAME, NewDriver())
}
