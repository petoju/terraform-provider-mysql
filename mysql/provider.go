package mysql

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"golang.org/x/net/proxy"
)

const (
	cleartextPasswords = "cleartext"
	nativePasswords    = "native"
	unknownVarErrCode  = 1193
	unknownUserErrCode = 1396
)

type MySQLConfiguration struct {
	Config                 *mysql.Config
	Db                     *sql.DB
	MaxConnLifetime        time.Duration
	MaxOpenConns           int
	ConnectRetryTimeoutSec time.Duration
	Version                *version.Version
}

var (
	connectionCacheMtx sync.Mutex
	connectionCache    map[string]*sql.DB
)

func init() {
	connectionCacheMtx.Lock()
	defer connectionCacheMtx.Unlock()

	connectionCache = map[string]*sql.DB{}
}

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("MYSQL_ENDPOINT", nil),
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					value := v.(string)
					if value == "" {
						errors = append(errors, fmt.Errorf("Endpoint must not be an empty string"))
					}

					return
				},
			},

			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("MYSQL_USERNAME", nil),
			},

			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("MYSQL_PASSWORD", nil),
			},

			"proxy": {
				Type:     schema.TypeString,
				Optional: true,
				DefaultFunc: schema.MultiEnvDefaultFunc([]string{
					"ALL_PROXY",
					"all_proxy",
				}, nil),
				ValidateFunc: validation.StringMatch(regexp.MustCompile("^socks5h?://.*:\\d+$"), "The proxy URL is not a valid socks url."),
			},

			"tls": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("MYSQL_TLS_CONFIG", "false"),
				ValidateFunc: validation.StringInSlice([]string{
					"true",
					"false",
					"skip-verify",
				}, false),
			},

			"max_conn_lifetime_sec": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"max_open_conns": {
				Type:     schema.TypeInt,
				Optional: true,
			},

			"conn_params": {
				Type:     schema.TypeMap,
				Optional: true,
				Default:  nil,
			},

			"authentication_plugin": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      nativePasswords,
				ValidateFunc: validation.StringInSlice([]string{cleartextPasswords, nativePasswords}, true),
			},

			"connect_retry_timeout_sec": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  300,
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"mysql_tables": dataSourceTables(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"mysql_database":        resourceDatabase(),
			"mysql_global_variable": resourceGlobalVariable(),
			"mysql_grant":           resourceGrant(),
			"mysql_role":            resourceRole(),
			"mysql_sql":             resourceSql(),
			"mysql_user_password":   resourceUserPassword(),
			"mysql_user":            resourceUser(),
			"mysql_ti_config":       resourceTiConfigVariable(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {

	var endpoint = d.Get("endpoint").(string)
	var conn_params = make(map[string]string)

	proto := "tcp"
	if len(endpoint) > 0 && endpoint[0] == '/' {
		proto = "unix"
	}

	for k, vint := range d.Get("conn_params").(map[string]interface{}) {
		v, ok := vint.(string)
		if !ok {
			return nil, fmt.Errorf("Cannot convert connection parameters to string")
		}
		conn_params[k] = v
	}

	conf := mysql.Config{
		User:                    d.Get("username").(string),
		Passwd:                  d.Get("password").(string),
		Net:                     proto,
		Addr:                    endpoint,
		TLSConfig:               d.Get("tls").(string),
		AllowNativePasswords:    d.Get("authentication_plugin").(string) == nativePasswords,
		AllowCleartextPasswords: d.Get("authentication_plugin").(string) == cleartextPasswords,
		InterpolateParams:       true,
		Params:                  conn_params,
	}

	dialer, err := makeDialer(d)
	if err != nil {
		return nil, err
	}

	mysql.RegisterDial("tcp", func(network string) (net.Conn, error) {
		return dialer.Dial("tcp", network)
	})

	mysqlConf := &MySQLConfiguration{
		Config:                 &conf,
		MaxConnLifetime:        time.Duration(d.Get("max_conn_lifetime_sec").(int)) * time.Second,
		MaxOpenConns:           d.Get("max_open_conns").(int),
		ConnectRetryTimeoutSec: time.Duration(d.Get("connect_retry_timeout_sec").(int)) * time.Second,
	}

	db, err := connectToMySQL(mysqlConf)

	if err != nil {
		return nil, err
	}

	mysqlConf.Db = db
	if err := afterConnect(mysqlConf, db); err != nil {
		return nil, fmt.Errorf("Failed running after connect command: %v", err)
	}

	return mysqlConf, nil
}

func afterConnect(mysqlConf *MySQLConfiguration, db *sql.DB) error {
	// Set up env so that we won't create users randomly.
	currentVersion, err := serverVersion(db)
	if err != nil {
		return fmt.Errorf("Failed getting server version: %v", err)
	}

	mysqlConf.Version = currentVersion

	versionMinInclusive, _ := version.NewVersion("5.7.5")
	versionMaxExclusive, _ := version.NewVersion("8.0.0")
	if mysqlConf.Version.GreaterThanOrEqual(versionMinInclusive) &&
		mysqlConf.Version.LessThan(versionMaxExclusive) {
		// CONCAT and setting works even if there is no value.
		_, err := db.Exec(`SET SESSION sql_mode=CONCAT(@@sql_mode, ',NO_AUTO_CREATE_USER')`)
		if err != nil {
			return fmt.Errorf("Failed setting SQL mode: %v", err)
		}
	}

	return nil
}

var identQuoteReplacer = strings.NewReplacer("`", "``")

func makeDialer(d *schema.ResourceData) (proxy.Dialer, error) {
	proxyFromEnv := proxy.FromEnvironment()
	proxyArg := d.Get("proxy").(string)

	if len(proxyArg) > 0 {
		proxyURL, err := url.Parse(proxyArg)
		if err != nil {
			return nil, err
		}
		proxy, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, err
		}

		return proxy, nil
	}

	return proxyFromEnv, nil
}

func quoteIdentifier(in string) string {
	return fmt.Sprintf("`%s`", identQuoteReplacer.Replace(in))
}

func serverVersion(db *sql.DB) (*version.Version, error) {
	var versionString string
	err := db.QueryRow("SELECT @@GLOBAL.innodb_version").Scan(&versionString)
	if err != nil {
		return nil, err
	}

	return version.NewVersion(versionString)
}

func serverVersionString(db *sql.DB) (string, error) {
	var versionString string
	err := db.QueryRow("SELECT @@GLOBAL.version").Scan(&versionString)
	if err != nil {
		return "", err
	}

	return versionString, nil
}

func connectToMySQL(conf *MySQLConfiguration) (*sql.DB, error) {
	// This is fine - we'll connect serially, but we don't expect more than
	// 1 or 2 connections starting at once.
	connectionCacheMtx.Lock()
	defer connectionCacheMtx.Unlock()

	dsn := conf.Config.FormatDSN()
	if connectionCache[dsn] != nil {
		return connectionCache[dsn], nil
	}
	var db *sql.DB
	var err error

	// When provisioning a database server there can often be a lag between
	// when Terraform thinks it's available and when it is actually available.
	// This is particularly acute when provisioning a server and then immediately
	// trying to provision a database on it.
	retryError := resource.Retry(conf.ConnectRetryTimeoutSec, func() *resource.RetryError {
		db, err = sql.Open("mysql", dsn)
		if err != nil {
			if mysqlErrorNumber(err) == unknownVarErrCode {
				return resource.NonRetryableError(err)
			}
			return resource.RetryableError(err)
		}

		err = db.Ping()
		if err != nil {
			if mysqlErrorNumber(err) == unknownVarErrCode {
				return resource.NonRetryableError(err)
			}

			return resource.RetryableError(err)
		}

		return nil
	})

	if retryError != nil {
		return nil, fmt.Errorf("Could not connect to server: %s", retryError)
	}
	connectionCache[dsn] = db
	db.SetConnMaxLifetime(conf.MaxConnLifetime)
	db.SetMaxOpenConns(conf.MaxOpenConns)
	return db, nil
}

// 0 == not mysql error or not error at all.
func mysqlErrorNumber(err error) uint16 {
	if err == nil {
		return 0
	}
	me, ok := err.(*mysql.MySQLError)
	if !ok {
		return 0
	}
	return me.Number
}
