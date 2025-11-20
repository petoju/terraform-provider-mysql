//go:build spike
// +build spike

package mysql

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MySQLTestContainer wraps a testcontainers MySQL container with connection details
type MySQLTestContainer struct {
	Container testcontainers.Container
	Endpoint  string
	Username  string
	Password  string
	cleanup   func()
}

// startMySQLContainer starts a MySQL/Percona/MariaDB container for testing
func startMySQLContainer(ctx context.Context, t *testing.T, image string) *MySQLTestContainer {
	container, err := mysql.RunContainer(ctx,
		testcontainers.WithImage(image),
		mysql.WithDatabase("testdb"),
		mysql.WithUsername("root"),
		mysql.WithPassword(""),
		testcontainers.WithWaitStrategy(
			wait.ForLog("ready for connections").
				WithOccurrence(2).
				WithStartupTimeout(120*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start MySQL container: %v", err)
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
	os.Setenv("MYSQL_ENDPOINT", m.Endpoint)
	os.Setenv("MYSQL_USERNAME", m.Username)
	os.Setenv("MYSQL_PASSWORD", m.Password)

	return func() {
		os.Unsetenv("MYSQL_ENDPOINT")
		os.Unsetenv("MYSQL_USERNAME")
		os.Unsetenv("MYSQL_PASSWORD")
		ctx := context.Background()
		if err := m.Container.Terminate(ctx); err != nil {
			t.Logf("Warning: Failed to terminate container: %v", err)
		}
	}
}

// Example usage in a test
func ExampleTestAccDatabase_WithTestcontainers(t *testing.T) {
	ctx := context.Background()
	container := startMySQLContainer(ctx, t, "mysql:8.0")
	defer container.SetupTestEnv(t)()

	// Now tests can use MYSQL_ENDPOINT, MYSQL_USERNAME, MYSQL_PASSWORD
	// Existing test logic works without changes
}
