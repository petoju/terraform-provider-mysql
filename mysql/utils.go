package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"google.golang.org/api/googleapi"
	"log"
	"sync"

	"github.com/hashicorp/go-version"
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
	mysqlConf := meta.(*MySQLConfiguration)
	oneConnection, err := connectToMySQLInternal(ctx, mysqlConf)

	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	return oneConnection.Db, nil
}

func getVersionFromMeta(ctx context.Context, meta interface{}) *version.Version {
	mysqlConf := meta.(*MySQLConfiguration)
	oneConnection, err := connectToMySQLInternal(ctx, mysqlConf)
	if err != nil {
		log.Panicf("getting DB got us error: %v", err)
	}

	return oneConnection.Version
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
