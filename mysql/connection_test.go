package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
)

func TestSetSQLModeParam_MySQL56(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("5.6.50")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "''" {
		t.Errorf("expected sql_mode='' for MySQL 5.6.50, got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL574(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("5.7.4")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "''" {
		t.Errorf("expected sql_mode='' for MySQL 5.7.4 (below 5.7.5), got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL575(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("5.7.5")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "'NO_AUTO_CREATE_USER'" {
		t.Errorf("expected sql_mode='NO_AUTO_CREATE_USER' for MySQL 5.7.5 (inclusive), got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL5742(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("5.7.42")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "'NO_AUTO_CREATE_USER'" {
		t.Errorf("expected sql_mode='NO_AUTO_CREATE_USER' for MySQL 5.7.42, got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL800(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("8.0.0")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "''" {
		t.Errorf("expected sql_mode='' for MySQL 8.0.0 (exclusive), got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL8x(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("8.0.35")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "''" {
		t.Errorf("expected sql_mode='' for MySQL 8.0.35, got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_MySQL84(t *testing.T) {
	params := make(map[string]string)
	v, _ := version.NewVersion("8.4.0")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "''" {
		t.Errorf("expected sql_mode='' for MySQL 8.4.0, got %s", params["sql_mode"])
	}
}

func TestSetSQLModeParam_PreservesExistingParams(t *testing.T) {
	params := map[string]string{
		"collation": "utf8_bin",
	}
	v, _ := version.NewVersion("5.7.42")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "'NO_AUTO_CREATE_USER'" {
		t.Errorf("expected sql_mode='NO_AUTO_CREATE_USER', got %s", params["sql_mode"])
	}
	if params["collation"] != "utf8_bin" {
		t.Errorf("expected collation=utf8_bin to be preserved, got %s", params["collation"])
	}
}

func TestSetSQLModeParam_OverwritesExistingSQLMode(t *testing.T) {
	params := map[string]string{
		"sql_mode": "ANSI_QUOTES",
	}
	v, _ := version.NewVersion("5.7.42")
	setSQLModeParam(params, v)
	if params["sql_mode"] != "'NO_AUTO_CREATE_USER'" {
		t.Errorf("expected sql_mode to be overwritten to 'NO_AUTO_CREATE_USER', got %s", params["sql_mode"])
	}
}

func TestConfigureConnectionPool_MaxOpenConnsPositive(t *testing.T) {
	db, err := sql.Open("mysql", "unused:unused@/")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	configureConnectionPool(db, 30*time.Second, 5)

	stats := db.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Errorf("expected MaxOpenConnections=5, got %d", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPool_MaxOpenConnsOne(t *testing.T) {
	db, err := sql.Open("mysql", "unused:unused@/")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	configureConnectionPool(db, 30*time.Second, 1)

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected MaxOpenConnections=1, got %d", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPool_MaxOpenConnsZero(t *testing.T) {
	db, err := sql.Open("mysql", "unused:unused@/")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	configureConnectionPool(db, 30*time.Second, 0)

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected MaxOpenConnections=1 (default when 0), got %d", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPool_MaxOpenConnsNegative(t *testing.T) {
	db, err := sql.Open("mysql", "unused:unused@/")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	configureConnectionPool(db, 30*time.Second, -1)

	stats := db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("expected MaxOpenConnections=1 (default when negative), got %d", stats.MaxOpenConnections)
	}
}

func TestConfigureConnectionPool_LargePool(t *testing.T) {
	db, err := sql.Open("mysql", "unused:unused@/")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	configureConnectionPool(db, 60*time.Second, 25)

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("expected MaxOpenConnections=25, got %d", stats.MaxOpenConnections)
	}
}

func TestMaxOpenConnsSchemaDefault(t *testing.T) {
	provider := Provider()
	s, ok := provider.Schema["max_open_conns"]
	if !ok {
		t.Fatal("max_open_conns schema not found")
	}
	if s.Default != 1 {
		t.Errorf("expected max_open_conns Default to be 1, got %v", s.Default)
	}
}

func TestConnectionCache_Initialized(t *testing.T) {
	connectionCacheMtx.Lock()
	if connectionCache == nil {
		t.Error("connectionCache should be initialized in init()")
	}
	connectionCacheMtx.Unlock()
}

func TestConnectionCache_StoreAndRetrieve(t *testing.T) {
	v, _ := version.NewVersion("8.0.0")
	testConn := &DbConnection{
		Version: v,
	}

	connectionCacheMtx.Lock()
	connectionCache["test-store-dsn"] = testConn
	retrieved := connectionCache["test-store-dsn"]
	delete(connectionCache, "test-store-dsn")
	connectionCacheMtx.Unlock()

	if retrieved != testConn {
		t.Error("expected to retrieve the same DbConnection pointer from cache")
	}
	if retrieved.Version != v {
		t.Error("expected version to match in cached DbConnection")
	}
}

func TestConnectionCache_CacheHit(t *testing.T) {
	v, _ := version.NewVersion("5.7.42")
	cachedConn := &DbConnection{
		Version: v,
	}

	conf := &MySQLConfiguration{
		Config: &mysql.Config{
			User:   "cacheuser",
			Passwd: "cachepass",
			Net:    "tcp",
			Addr:   "localhost:3306",
		},
		MaxOpenConns:           1,
		ConnectRetryTimeoutSec: 1 * time.Second,
	}
	dsn := conf.Config.FormatDSN()

	connectionCacheMtx.Lock()
	connectionCache[dsn] = cachedConn
	connectionCacheMtx.Unlock()

	conn, err := connectToMySQLInternal(context.Background(), conf)
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if conn != cachedConn {
		t.Error("expected cached DbConnection to be returned on cache hit")
	}

	connectionCacheMtx.Lock()
	delete(connectionCache, dsn)
	connectionCacheMtx.Unlock()
}

func TestConnectionCache_DifferentDSNs(t *testing.T) {
	v1, _ := version.NewVersion("5.7.42")
	v2, _ := version.NewVersion("8.0.35")
	conn1 := &DbConnection{Version: v1}
	conn2 := &DbConnection{Version: v2}

	conf1 := &MySQLConfiguration{
		Config: &mysql.Config{
			User: "user1", Passwd: "pass1", Net: "tcp", Addr: "host1:3306",
		},
	}
	conf2 := &MySQLConfiguration{
		Config: &mysql.Config{
			User: "user2", Passwd: "pass2", Net: "tcp", Addr: "host2:3306",
		},
	}
	dsn1 := conf1.Config.FormatDSN()
	dsn2 := conf2.Config.FormatDSN()

	connectionCacheMtx.Lock()
	connectionCache[dsn1] = conn1
	connectionCache[dsn2] = conn2

	if connectionCache[dsn1] != conn1 {
		t.Error("expected conn1 for dsn1")
	}
	if connectionCache[dsn2] != conn2 {
		t.Error("expected conn2 for dsn2")
	}

	delete(connectionCache, dsn1)
	delete(connectionCache, dsn2)
	connectionCacheMtx.Unlock()
}
