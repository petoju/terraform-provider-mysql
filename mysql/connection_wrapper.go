package mysql

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sync"

	"github.com/hashicorp/go-version"
)

// mysqlConnInitializer wraps driver.Conn and ensures session settings are applied
// to each connection when it's first used from the pool.
type mysqlConnInitializer struct {
	conn        driver.Conn
	initialized bool
	mu          sync.Mutex
	version     *version.Version
}

func (c *mysqlConnInitializer) Prepare(query string) (driver.Stmt, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	return c.conn.Prepare(query)
}

func (c *mysqlConnInitializer) Close() error {
	return c.conn.Close()
}

func (c *mysqlConnInitializer) Begin() (driver.Tx, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	return c.conn.Begin()
}

func (c *mysqlConnInitializer) ensureInit() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized {
		return nil
	}
	c.initialized = true
	return nil
}

// Execer interface for executing queries
//
// Deprecated: Drivers should implement [ExecContext] instead.
func (c *mysqlConnInitializer) Exec(query string, args []driver.Value) (driver.Result, error) {
	// This is called for ExecContext internally, we need to use the raw conn if it supports Exec
	if execer, ok := c.conn.(driver.Execer); ok {
		return execer.Exec(query, args)
	}
	return nil, driver.ErrSkip
}

// Queryer interface for querying
//
// Deprecated: Drivers should implement [QueryContext] instead.
func (c *mysqlConnInitializer) Query(query string, args []driver.Value) (driver.Rows, error) {
	if queryer, ok := c.conn.(driver.Queryer); ok {
		return queryer.Query(query, args)
	}
	return nil, driver.ErrSkip
}

// ConnPrepareContext for preparing statements with context
func (c *mysqlConnInitializer) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	if prepCtx, ok := c.conn.(driver.ConnPrepareContext); ok {
		return prepCtx.PrepareContext(ctx, query)
	}
	return c.Prepare(query)
}

// ConnBeginTx for beginning transactions with context
func (c *mysqlConnInitializer) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := c.ensureInit(); err != nil {
		return nil, err
	}
	if beginTx, ok := c.conn.(driver.ConnBeginTx); ok {
		return beginTx.BeginTx(ctx, opts)
	}
	return c.Begin()
}

// ExecerContext for executing with context
func (c *mysqlConnInitializer) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execCtx, ok := c.conn.(driver.ExecerContext); ok {
		return execCtx.ExecContext(ctx, query, args)
	}
	// Fallback to Exec if no context version available
	dargs := make([]driver.Value, len(args))
	for i, arg := range args {
		dargs[i] = arg.Value
	}
	return c.Exec(query, dargs)
}

// QueryerContext for querying with context
func (c *mysqlConnInitializer) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryCtx, ok := c.conn.(driver.QueryerContext); ok {
		return queryCtx.QueryContext(ctx, query, args)
	}
	// Fallback to Query if no context version available
	dargs := make([]driver.Value, len(args))
	for i, arg := range args {
		dargs[i] = arg.Value
	}
	return c.Query(query, dargs)
}

// Ping for connection health checks
func (c *mysqlConnInitializer) Ping(ctx context.Context) error {
	if pinger, ok := c.conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

// CheckNamedValue for named value handling
func (c *mysqlConnInitializer) CheckNamedValue(nv *driver.NamedValue) error {
	if checker, ok := c.conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

// SessionResetter for resetting session state when connection is returned to pool
func (c *mysqlConnInitializer) ResetSession(ctx context.Context) error {
	if resetter, ok := c.conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

// mysqlConnectorWrapper wraps the MySQL connector to apply session settings to each new connection
type mysqlConnectorWrapper struct {
	connector   driver.Connector
	version     *version.Version
	versionMu   sync.RWMutex
	versionOnce sync.Once
}

func (c *mysqlConnectorWrapper) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return &mysqlConnInitializer{
		conn:    conn,
		version: c.getVersion(),
	}, nil
}

func (c *mysqlConnectorWrapper) Driver() driver.Driver {
	return c.connector.Driver()
}

func (c *mysqlConnectorWrapper) getVersion() *version.Version {
	c.versionMu.RLock()
	defer c.versionMu.RUnlock()
	return c.version
}

// sessionInitializingConnector wraps a driver.Connector to initialize session settings
// on each new connection that is created from the pool.
type sessionInitializingConnector struct {
	base         driver.Connector
	sqlModeQuery string
	version      *version.Version
	versionMu    sync.RWMutex
}

func (c *sessionInitializingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}

	// Get the version once
	c.versionMu.RLock()
	ver := c.version
	c.versionMu.RUnlock()

	if ver != nil && c.sqlModeQuery != "" {
		// Apply session settings to the connection
		if execer, ok := conn.(driver.ExecerContext); ok {
			_, err = execer.ExecContext(ctx, c.sqlModeQuery, nil)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to set session sql_mode: %v", err)
			}
		} else if execerOld, ok := conn.(driver.Execer); ok {
			_, err = execerOld.Exec(c.sqlModeQuery, nil)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to set session sql_mode: %v", err)
			}
		}
	}

	return conn, nil
}

func (c *sessionInitializingConnector) Driver() driver.Driver {
	return c.base.Driver()
}

func (c *sessionInitializingConnector) setVersion(v *version.Version) {
	c.versionMu.Lock()
	defer c.versionMu.Unlock()
	c.version = v

	// Update the SQL mode query based on version
	versionMinInclusive, _ := version.NewVersion("5.7.5")
	versionMaxExclusive, _ := version.NewVersion("8.0.0")
	if v.GreaterThanOrEqual(versionMinInclusive) && v.LessThan(versionMaxExclusive) {
		// We set NO_AUTO_CREATE_USER to prevent provider from creating user when creating grants. Newer MySQL has it automatically.
		// We don't want any other modes, esp. not ANSI_QUOTES.
		c.sqlModeQuery = `SET SESSION sql_mode='NO_AUTO_CREATE_USER'`
	} else {
		// We don't want any modes, esp. not ANSI_QUOTES.
		c.sqlModeQuery = `SET SESSION sql_mode=''`
	}
}
