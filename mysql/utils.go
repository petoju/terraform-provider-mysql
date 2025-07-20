package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/go-sql-driver/mysql"
	"google.golang.org/api/googleapi"

	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/hashicorp/go-version"
	rds "github.com/krotscheck/go-rds-driver"
)

type KeyedMutex struct {
	mu    sync.Mutex // Protects access to the internal map
	locks map[string]*sync.Mutex
}

func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{
		locks: make(map[string]*sync.Mutex),
	}
}

func (km *KeyedMutex) Lock(key string) {
	km.mu.Lock()
	lock, exists := km.locks[key]
	if !exists {
		lock = &sync.Mutex{}
		km.locks[key] = lock
	}
	km.mu.Unlock()

	lock.Lock()
}

func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	lock, exists := km.locks[key]
	if !exists {
		panic("unlock of unlocked mutex")
	}
	km.mu.Unlock()

	lock.Unlock()
}

func hashSum(contents interface{}) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(contents.(string))))
}

func getDatabaseFromMeta(ctx context.Context, meta interface{}) (*sql.DB, error) {
	switch conf := meta.(type) {
	case *MySQLConfiguration:
		oneConnection, err := connectToMySQLInternal(ctx, conf)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL: %v", err)
		}
		return oneConnection.Db, nil

	case *RDSDataAPIConfiguration:
		return connectToRDSDataAPI(ctx, conf)

	default:
		return nil, fmt.Errorf("unexpected configuration type: %T", meta)
	}
}

func getVersionFromMeta(ctx context.Context, meta interface{}) *version.Version {
	switch conf := meta.(type) {
	case *MySQLConfiguration:
		oneConnection, err := connectToMySQLInternal(ctx, conf)
		if err != nil {
			log.Panicf("getting DB got us error: %v", err)
		}
		return oneConnection.Version

	case *RDSDataAPIConfiguration:
		db, err := getDatabaseFromMeta(ctx, meta)
		if err != nil {
			log.Panicf("getting DB got us error: %v", err)
		}

		ver, err := serverVersion(db)
		if err != nil {
			log.Panicf("getting version got us error: %v", err)
		}

		return ver

	default:
		log.Panicf("unexpected configuration type: %T", meta)
		return nil
	}
}

// 0 == not mysql error or not error at all.
func mysqlErrorNumber(err error) uint16 {
	if err == nil {
		return 0
	}
	var mysqlError *mysql.MySQLError
	ok := errors.As(err, &mysqlError)
	if !ok {
		return 0
	}
	return mysqlError.Number
}

func cloudsqlErrorNumber(err error) int {
	if err == nil {
		return 0
	}

	var gapiError *googleapi.Error
	if errors.As(err, &gapiError) {
		if gapiError.Code >= 400 && gapiError.Code < 500 {
			return gapiError.Code
		}
	}
	return 0
}

func connectToRDSDataAPI(ctx context.Context, conf *RDSDataAPIConfiguration) (*sql.DB, error) {
	rdsConnector := rds.NewConnector(rds.NewDriver(), rdsdata.NewFromConfig(conf.AWSConfig), conf.Config)

	db := sql.OpenDB(rdsConnector)

	err := db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping RDS Data API: %v", err)
	}

	return db, nil
}
