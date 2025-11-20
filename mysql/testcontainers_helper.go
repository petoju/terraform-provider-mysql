//go:build testcontainers
// +build testcontainers

// Suppress warnings from go-m1cpu dependency
// These are harmless compiler warnings from CGO code in a third-party package

package mysql

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	sharedContainer     *MySQLTestContainer
	sharedContainerOnce sync.Once
	sharedContainerMtx  sync.Mutex

	sharedTiDBCluster     *TiDBTestCluster
	sharedTiDBClusterOnce sync.Once
	sharedTiDBClusterMtx  sync.Mutex
)

func init() {
	// Suppress MySQL driver "unexpected EOF" log messages during tests
	// These are harmless connection cleanup messages that occur when connections
	// are closed during test cleanup
	mysql.SetLogger(log.New(&mysqlLogFilter{Writer: io.Discard}, "", log.LstdFlags))
}

// mysqlLogFilter filters out "unexpected EOF" messages from MySQL driver logs
type mysqlLogFilter struct {
	io.Writer
}

func (f *mysqlLogFilter) Write(p []byte) (n int, err error) {
	// Filter out "unexpected EOF" messages
	if strings.Contains(string(p), "unexpected EOF") {
		return len(p), nil // Discard the message
	}
	return len(p), nil // Also discard other messages to suppress all MySQL driver logging
}

// MySQLTestContainer wraps a testcontainers MySQL container with connection details
type MySQLTestContainer struct {
	Container testcontainers.Container
	Endpoint  string
	Username  string
	Password  string
}

// startMySQLContainer starts a MySQL/Percona/MariaDB container for testing
// Supports MySQL, Percona, and MariaDB images
func startMySQLContainer(ctx context.Context, t *testing.T, image string) *MySQLTestContainer {
	// Determine timeout based on image/version
	timeout := 120 * time.Second
	if contains(image, "5.6") || contains(image, "5.7") || contains(image, "6.1") || contains(image, "6.5") {
		// Older versions may need more time
		timeout = 180 * time.Second
	}

	// Use GenericContainer for compatibility with Go 1.21
	// Configure MySQL with environment variables
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD":        "",
			"MYSQL_ALLOW_EMPTY_PASSWORD": "1",
			"MYSQL_DATABASE":             "testdb",
		},
		WaitingFor: wait.ForLog("ready for connections").
			WithOccurrence(2).
			WithStartupTimeout(timeout),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start MySQL container (%s): %v", image, err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "3306")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	return &MySQLTestContainer{
		Container: container,
		Endpoint:  endpoint,
		Username:  "root",
		Password:  "",
	}
}

// SetupTestEnv sets environment variables for the test and returns a cleanup function
func (m *MySQLTestContainer) SetupTestEnv(t *testing.T) func() {
	originalEndpoint := os.Getenv("MYSQL_ENDPOINT")
	originalUsername := os.Getenv("MYSQL_USERNAME")
	originalPassword := os.Getenv("MYSQL_PASSWORD")

	os.Setenv("MYSQL_ENDPOINT", m.Endpoint)
	os.Setenv("MYSQL_USERNAME", m.Username)
	os.Setenv("MYSQL_PASSWORD", m.Password)

	return func() {
		// Restore original values or unset
		if originalEndpoint != "" {
			os.Setenv("MYSQL_ENDPOINT", originalEndpoint)
		} else {
			os.Unsetenv("MYSQL_ENDPOINT")
		}
		if originalUsername != "" {
			os.Setenv("MYSQL_USERNAME", originalUsername)
		} else {
			os.Unsetenv("MYSQL_USERNAME")
		}
		if originalPassword != "" {
			os.Setenv("MYSQL_PASSWORD", originalPassword)
		} else {
			os.Unsetenv("MYSQL_PASSWORD")
		}

		// Terminate container
		ctx := context.Background()
		if err := m.Container.Terminate(ctx); err != nil {
			t.Logf("Warning: Failed to terminate container: %v", err)
		}
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// getSharedMySQLContainer returns a shared MySQL container for all tests
// The container is created once and reused across all tests in the package
func getSharedMySQLContainer(t *testing.T, image string) *MySQLTestContainer {
	sharedContainerOnce.Do(func() {
		ctx := context.Background()
		sharedContainer = startMySQLContainer(ctx, t, image)

		// Set up environment variables for the shared container
		os.Setenv("MYSQL_ENDPOINT", sharedContainer.Endpoint)
		os.Setenv("MYSQL_USERNAME", sharedContainer.Username)
		os.Setenv("MYSQL_PASSWORD", sharedContainer.Password)
	})
	return sharedContainer
}

// startSharedMySQLContainer starts a shared MySQL container without requiring a testing.T
// Used by TestMain for initial setup
func startSharedMySQLContainer(image string) (*MySQLTestContainer, error) {
	ctx := context.Background()

	// Determine timeout based on image/version
	timeout := 120 * time.Second
	if contains(image, "5.6") || contains(image, "5.7") || contains(image, "6.1") || contains(image, "6.5") {
		timeout = 180 * time.Second
	}

	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD":        "",
			"MYSQL_ALLOW_EMPTY_PASSWORD": "1",
			"MYSQL_DATABASE":             "testdb",
		},
		WaitingFor: wait.ForLog("ready for connections").
			WithOccurrence(2).
			WithStartupTimeout(timeout),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start MySQL container (%s): %v", image, err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "3306")
	if err != nil {
		return nil, fmt.Errorf("failed to get container port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	mysqlContainer := &MySQLTestContainer{
		Container: container,
		Endpoint:  endpoint,
		Username:  "root",
		Password:  "",
	}

	// Install mysql_no_login plugin (required for some tests)
	// This matches what the Makefile does
	// Wait a bit for MySQL to be fully ready before installing plugin
	time.Sleep(2 * time.Second)
	if err := installMySQLNoLoginPlugin(ctx, mysqlContainer); err != nil {
		// Log warning but don't fail - plugin may already be installed or not available
		// Some MySQL versions/distributions may not have this plugin
		fmt.Printf("Warning: Could not install mysql_no_login plugin: %v (some tests may skip)\n", err)
	}

	return mysqlContainer, nil
}

// installMySQLNoLoginPlugin installs the mysql_no_login plugin in the container
func installMySQLNoLoginPlugin(ctx context.Context, container *MySQLTestContainer) error {
	// Connect to MySQL and install the plugin
	db, err := connectToMySQL(ctx, &MySQLConfiguration{
		Config: &mysql.Config{
			User:   container.Username,
			Passwd: container.Password,
			Net:    "tcp",
			Addr:   container.Endpoint,
		},
		MaxConnLifetime:        0,
		MaxOpenConns:           1,
		ConnectRetryTimeoutSec: 30 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to install plugin: %v", err)
	}
	defer db.Close()

	// Try to install the plugin (ignore error if already installed or not available)
	_, err = db.ExecContext(ctx, "INSTALL PLUGIN mysql_no_login SONAME 'mysql_no_login.so'")
	if err != nil {
		errStr := err.Error()
		// Ignore if plugin already exists or if plugin file doesn't exist (MySQL 8.0 may not have it)
		if strings.Contains(errStr, "already exists") ||
			strings.Contains(errStr, "file not found") ||
			strings.Contains(errStr, "does not exist") {
			return nil // Not an error - plugin already installed or not available
		}
		return fmt.Errorf("failed to install mysql_no_login plugin: %v", err)
	}

	return nil
}

// cleanupSharedContainer terminates the shared container
func cleanupSharedContainer() {
	sharedContainerMtx.Lock()
	defer sharedContainerMtx.Unlock()

	if sharedContainer != nil {
		ctx := context.Background()
		if err := sharedContainer.Container.Terminate(ctx); err != nil {
			// Use fmt.Printf since we're in cleanup and testing.T may not be available
			fmt.Printf("Warning: Failed to terminate shared container: %v\n", err)
		}
		sharedContainer = nil
	}

	// Clean up environment variables
	os.Unsetenv("MYSQL_ENDPOINT")
	os.Unsetenv("MYSQL_USERNAME")
	os.Unsetenv("MYSQL_PASSWORD")
}

// TiDBTestCluster wraps TiDB cluster containers with connection details
type TiDBTestCluster struct {
	PDContainer   testcontainers.Container
	TiKVContainer testcontainers.Container
	TiDBContainer testcontainers.Container
	Network       testcontainers.Network
	Endpoint      string
	Username      string
	Password      string
}

// startTiDBCluster starts a TiDB cluster (PD, TiKV, TiDB) for testing
// TiDB requires a multi-container setup with coordination between components
func startTiDBCluster(ctx context.Context, t *testing.T, version string) *TiDBTestCluster {
	// Create a Docker network for TiDB cluster components
	testNetwork, err := network.New(ctx,
		network.WithCheckDuplicate(),
		network.WithDriver("bridge"),
	)
	if err != nil {
		t.Fatalf("Failed to create Docker network: %v", err)
	}

	networkName := testNetwork.Name

	// Start PD (Placement Driver) - must start first
	pdContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/pd:v%s", version),
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"pd"}},
			Cmd: []string{
				"--name=pd",
				"--data-dir=/data",
				"--client-urls=http://0.0.0.0:2379",
				"--advertise-client-urls=http://pd:2379",
				"--peer-urls=http://0.0.0.0:2380",
				"--advertise-peer-urls=http://pd:2380",
				"--initial-cluster=pd=http://pd:2380",
			},
			WaitingFor: wait.ForLog("ready to serve").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start PD container: %v", err)
	}

	// Start TiKV (storage layer) - connects to PD
	tikvContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/tikv:v%s", version),
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"tikv"}},
			Cmd: []string{
				"--addr=0.0.0.0:20160",
				"--advertise-addr=tikv:20160",
				"--status-addr=0.0.0.0:20180",
				"--data-dir=/data",
				"--pd=pd:2379",
			},
			WaitingFor: wait.ForLog("TiKV started").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start TiKV container: %v", err)
	}

	// Start TiDB (SQL layer) - connects to PD, uses TiKV for storage
	tidbContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/tidb:v%s", version),
			ExposedPorts:   []string{"4000/tcp"},
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"tidb"}},
			Cmd: []string{
				"--store=tikv",
				"-P", "4000",
				"--path=pd:2379",
			},
			WaitingFor: wait.ForLog("server is running MySQL protocol").
				WithOccurrence(1).
				WithStartupTimeout(180 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start TiDB container: %v", err)
	}

	// Get TiDB endpoint
	host, err := tidbContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get TiDB container host: %v", err)
	}

	port, err := tidbContainer.MappedPort(ctx, "4000")
	if err != nil {
		t.Fatalf("Failed to get TiDB container port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	return &TiDBTestCluster{
		PDContainer:   pdContainer,
		TiKVContainer: tikvContainer,
		TiDBContainer: tidbContainer,
		Network:       testNetwork,
		Endpoint:      endpoint,
		Username:      "root",
		Password:      "",
	}
}

// startSharedTiDBCluster starts a shared TiDB cluster without requiring a testing.T
// Used by TestMain for initial setup
func startSharedTiDBCluster(version string) (*TiDBTestCluster, error) {
	ctx := context.Background()

	// Create a Docker network for TiDB cluster components
	testNetwork, err := network.New(ctx,
		network.WithCheckDuplicate(),
		network.WithDriver("bridge"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker network: %v", err)
	}

	networkName := testNetwork.Name

	// Start PD (Placement Driver) - must start first
	pdContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/pd:v%s", version),
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"pd"}},
			Cmd: []string{
				"--name=pd",
				"--data-dir=/data",
				"--client-urls=http://0.0.0.0:2379",
				"--advertise-client-urls=http://pd:2379",
				"--peer-urls=http://0.0.0.0:2380",
				"--advertise-peer-urls=http://pd:2380",
				"--initial-cluster=pd=http://pd:2380",
			},
			WaitingFor: wait.ForLog("ready to serve").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start PD container: %v", err)
	}

	// Start TiKV (storage layer) - connects to PD
	tikvContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/tikv:v%s", version),
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"tikv"}},
			Cmd: []string{
				"--addr=0.0.0.0:20160",
				"--advertise-addr=tikv:20160",
				"--status-addr=0.0.0.0:20180",
				"--data-dir=/data",
				"--pd=pd:2379",
			},
			WaitingFor: wait.ForLog("TiKV started").
				WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start TiKV container: %v", err)
	}

	// Start TiDB (SQL layer) - connects to PD, uses TiKV for storage
	tidbContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          fmt.Sprintf("pingcap/tidb:v%s", version),
			ExposedPorts:   []string{"4000/tcp"},
			Networks:       []string{networkName},
			NetworkAliases: map[string][]string{networkName: {"tidb"}},
			Cmd: []string{
				"--store=tikv",
				"-P", "4000",
				"--path=pd:2379",
			},
			WaitingFor: wait.ForLog("server is running MySQL protocol").
				WithOccurrence(1).
				WithStartupTimeout(180 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start TiDB container: %v", err)
	}

	// Get TiDB endpoint
	host, err := tidbContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB container host: %v", err)
	}

	port, err := tidbContainer.MappedPort(ctx, "4000")
	if err != nil {
		return nil, fmt.Errorf("failed to get TiDB container port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	return &TiDBTestCluster{
		PDContainer:   pdContainer,
		TiKVContainer: tikvContainer,
		TiDBContainer: tidbContainer,
		Network:       testNetwork,
		Endpoint:      endpoint,
		Username:      "root",
		Password:      "",
	}, nil
}

// cleanupSharedTiDBCluster terminates the shared TiDB cluster
func cleanupSharedTiDBCluster() {
	sharedTiDBClusterMtx.Lock()
	defer sharedTiDBClusterMtx.Unlock()

	if sharedTiDBCluster != nil {
		ctx := context.Background()
		if err := sharedTiDBCluster.TiDBContainer.Terminate(ctx); err != nil {
			fmt.Printf("Warning: Failed to terminate TiDB container: %v\n", err)
		}
		if err := sharedTiDBCluster.TiKVContainer.Terminate(ctx); err != nil {
			fmt.Printf("Warning: Failed to terminate TiKV container: %v\n", err)
		}
		if err := sharedTiDBCluster.PDContainer.Terminate(ctx); err != nil {
			fmt.Printf("Warning: Failed to terminate PD container: %v\n", err)
		}
		if err := sharedTiDBCluster.Network.Remove(ctx); err != nil {
			fmt.Printf("Warning: Failed to remove TiDB network: %v\n", err)
		}
		sharedTiDBCluster = nil
	}

	// Clean up environment variables
	os.Unsetenv("MYSQL_ENDPOINT")
	os.Unsetenv("MYSQL_USERNAME")
	os.Unsetenv("MYSQL_PASSWORD")
}
