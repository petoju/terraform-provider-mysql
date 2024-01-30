package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
	"google.golang.org/api/googleapi"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"golang.org/x/net/proxy"
	"golang.org/x/oauth2"

	cloudsqlconn "cloud.google.com/go/cloudsqlconn"
	cloudsql "cloud.google.com/go/cloudsqlconn/mysql/mysql"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	cleartextPasswords = "cleartext"
	nativePasswords    = "native"
	unknownUserErrCode = 1396
)

type OneConnection struct {
	Db      *sql.DB
	Version *version.Version
}

type MySQLConfiguration struct {
	Config                 *mysql.Config
	MaxConnLifetime        time.Duration
	MaxOpenConns           int
	ConnectRetryTimeoutSec time.Duration
}

var (
	connectionCacheMtx sync.Mutex
	connectionCache    map[string]*OneConnection
)

func init() {
	connectionCacheMtx.Lock()
	defer connectionCacheMtx.Unlock()

	connectionCache = map[string]*OneConnection{}
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

			"iam_database_authentication": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
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
			"mysql_rds_config":      resourceRDSConfig(),
		},

		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	var endpoint = d.Get("endpoint").(string)
	var connParams = make(map[string]string)
	var authPlugin = d.Get("authentication_plugin").(string)
	var allowClearTextPasswords = authPlugin == cleartextPasswords
	var allowNativePasswords = authPlugin == nativePasswords
	var password = d.Get("password").(string)
	var iam_auth = d.Get("iam_database_authentication").(bool)

	proto := "tcp"
	if len(endpoint) > 0 && endpoint[0] == '/' {
		proto = "unix"
	} else if strings.HasPrefix(endpoint, "cloudsql://") {
		proto = "cloudsql"
		endpoint = strings.ReplaceAll(endpoint, "cloudsql://", "")
		var err error
		if iam_auth { // Access token will be in the password field
			token := oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: password,
			})
			_, err = cloudsql.RegisterDriver("cloudsql", cloudsqlconn.WithIAMAuthNTokenSources(token, token))
		} else {
			_, err = cloudsql.RegisterDriver("cloudsql")
		}
		if err != nil {
			return nil, diag.Errorf("failed to register driver %v", err)
		}

	} else if strings.HasPrefix(endpoint, "azure://") {
		// Azure AD does not support native password authentication but go-sql-driver/mysql
		// has to be configured only with ?allowClearTextPasswords=true not with allowNativePasswords=false in this case
		allowClearTextPasswords = true
		azCredential, err := azidentity.NewDefaultAzureCredential(nil)
		endpoint = strings.ReplaceAll(endpoint, "azure://", "")

		if err != nil {
			return nil, diag.Errorf("failed to create Azure credential %v", err)
		}

		azToken, err := azCredential.GetToken(
			ctx,
			policy.TokenRequestOptions{Scopes: []string{"https://ossrdbms-aad.database.windows.net/.default"}},
		)

		if err != nil {
			return nil, diag.Errorf("failed to get token from Azure AD %v", err)
		}

		password = azToken.Token
	}

	for k, vint := range d.Get("conn_params").(map[string]interface{}) {
		v, ok := vint.(string)
		if !ok {
			return nil, diag.Errorf("cannot convert connection parameters to string")
		}
		connParams[k] = v
	}

	conf := mysql.Config{
		User:                    d.Get("username").(string),
		Passwd:                  password,
		Net:                     proto,
		Addr:                    endpoint,
		TLSConfig:               d.Get("tls").(string),
		AllowNativePasswords:    allowNativePasswords,
		AllowCleartextPasswords: allowClearTextPasswords,
		InterpolateParams:       true,
		Params:                  connParams,
	}

	dialer, err := makeDialer(d)
	if err != nil {
		return nil, diag.Errorf("failed making dialer: %v", err)
	}

	mysql.RegisterDialContext("tcp", func(ctx context.Context, network string) (net.Conn, error) {
		return dialer.Dial("tcp", network)
	})

	mysqlConf := &MySQLConfiguration{
		Config:                 &conf,
		MaxConnLifetime:        time.Duration(d.Get("max_conn_lifetime_sec").(int)) * time.Second,
		MaxOpenConns:           d.Get("max_open_conns").(int),
		ConnectRetryTimeoutSec: time.Duration(d.Get("connect_retry_timeout_sec").(int)) * time.Second,
	}

	return mysqlConf, nil
}

func afterConnectVersion(ctx context.Context, mysqlConf *MySQLConfiguration, db *sql.DB) (*version.Version, error) {
	// Set up env so that we won't create users randomly.
	currentVersion, err := serverVersion(db)
	if err != nil {
		return nil, fmt.Errorf("failed getting server version: %v", err)
	}

	versionMinInclusive, _ := version.NewVersion("5.7.5")
	versionMaxExclusive, _ := version.NewVersion("8.0.0")
	if currentVersion.GreaterThanOrEqual(versionMinInclusive) &&
		currentVersion.LessThan(versionMaxExclusive) {
		// CONCAT and setting works even if there is no value.
		_, err = db.ExecContext(ctx, `SET SESSION sql_mode=CONCAT(@@sql_mode, ',NO_AUTO_CREATE_USER')`)
		if err != nil {
			return nil, fmt.Errorf("failed setting SQL mode: %v", err)
		}
	}

	return currentVersion, nil
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
		proxyDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, err
		}

		return proxyDialer, nil
	}

	return proxyFromEnv, nil
}

func quoteIdentifier(in string) string {
	return fmt.Sprintf("`%s`", identQuoteReplacer.Replace(in))
}

func serverVersion(db *sql.DB) (*version.Version, error) {
	var versionString string
	err := db.QueryRow("SELECT @@GLOBAL.version").Scan(&versionString)
	if err != nil {
		return nil, err
	}

	versionString = strings.SplitN(versionString, ":", 2)[0]
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

func serverRds(db *sql.DB) (bool, error) {
	var metadataVersionString string
	err := db.QueryRow("SELECT @@GLOBAL.datadir").Scan(&metadataVersionString)
	if err != nil {
		return false, err
	}

	if strings.Contains(metadataVersionString, "rds") {
		return true, nil
	}

	return false, nil
}

func connectToMySQL(ctx context.Context, conf *MySQLConfiguration) (*sql.DB, error) {
	conn, err := connectToMySQLInternal(ctx, conf)
	if err != nil {
		return nil, err
	}
	return conn.Db, nil
}

func connectToMySQLInternal(ctx context.Context, conf *MySQLConfiguration) (*OneConnection, error) {
	// This is fine - we'll connect serially, but we don't expect more than
	// 1 or 2 connections starting at once.
	connectionCacheMtx.Lock()
	defer connectionCacheMtx.Unlock()

	dsn := conf.Config.FormatDSN()
	if connectionCache[dsn] != nil {
		return connectionCache[dsn], nil
	}

	connection, err := createNewConnection(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("could not create new connection: %v", err)
	}

	connectionCache[dsn] = connection
	return connectionCache[dsn], nil
}

func createNewConnection(ctx context.Context, conf *MySQLConfiguration) (*OneConnection, error) {
	var db *sql.DB
	var err error

	driverName := "mysql"
	if conf.Config.Net == "cloudsql" {
		driverName = "cloudsql"
	}
	log.Printf("[DEBUG] Using driverName: %s", driverName)

	// When provisioning a database server there can often be a lag between
	// when Terraform thinks it's available and when it is actually available.
	// This is particularly acute when provisioning a server and then immediately
	// trying to provision a database on it.
	retryError := resource.RetryContext(ctx, conf.ConnectRetryTimeoutSec, func() *resource.RetryError {
		db, err = sql.Open(driverName, conf.Config.FormatDSN())
		if err != nil {
			if mysqlErrorNumber(err) != 0 || cloudsqlErrorNumber(err) != 0 || ctx.Err() != nil {
				return resource.NonRetryableError(err)
			}
			return resource.RetryableError(err)
		}

		err = db.PingContext(ctx)
		if err != nil {
			if mysqlErrorNumber(err) != 0 || cloudsqlErrorNumber(err) != 0 || ctx.Err() != nil {
				return resource.NonRetryableError(err)
			}

			return resource.RetryableError(err)
		}

		return nil
	})

	if retryError != nil {
		return nil, fmt.Errorf("could not connect to server: %s", retryError)
	}
	db.SetConnMaxLifetime(conf.MaxConnLifetime)
	db.SetMaxOpenConns(conf.MaxOpenConns)

	currentVersion, err := afterConnectVersion(ctx, conf, db)
	if err != nil {
		return nil, fmt.Errorf("failed running after connect command: %v", err)
	}

	return &OneConnection{
		Db:      db,
		Version: currentVersion,
	}, nil
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
