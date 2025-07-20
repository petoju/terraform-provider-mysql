package rds

import "fmt"

// ErrNoMixedParams is thrown if parameters are mixed
var ErrNoMixedParams = fmt.Errorf("please do not mix ordinal and named parameters")

// ErrClosed indicates that the connection is closed
var ErrClosed = fmt.Errorf("this connection is closed")

// ErrInvalidDSNScheme for when the dsn doesn't match rds://
var ErrInvalidDSNScheme = fmt.Errorf("this driver requires a DSN scheme of rds://")
