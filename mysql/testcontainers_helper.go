// +build testcontainers

// Suppress warnings from go-m1cpu dependency
// These are harmless compiler warnings from CGO code in a third-party package
//go:build testcontainers

package mysql

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

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
			"MYSQL_ROOT_PASSWORD": "",
			"MYSQL_ALLOW_EMPTY_PASSWORD": "1",
			"MYSQL_DATABASE": "testdb",
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
