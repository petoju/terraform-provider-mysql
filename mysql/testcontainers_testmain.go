//go:build testcontainers
// +build testcontainers

package mysql

import (
	"fmt"
	"os"
	"testing"
)

// TestMain sets up a shared MySQL/TiDB container for all testcontainers tests
// This is more efficient than starting a container for each test
func TestMain(m *testing.M) {
	// Check if we're testing TiDB (requires multi-container setup)
	tidbVersion := os.Getenv("TIDB_VERSION")
	if tidbVersion != "" {
		// Start shared TiDB cluster before running tests
		var err error
		sharedTiDBClusterMtx.Lock()
		sharedTiDBCluster, err = startSharedTiDBCluster(tidbVersion)
		sharedTiDBClusterMtx.Unlock()

		if err != nil {
			// If cluster startup fails, exit with error
			os.Stderr.WriteString(fmt.Sprintf("Failed to start shared TiDB cluster: %v\n", err))
			os.Exit(1)
		}

		// Set up environment variables for the shared TiDB cluster
		os.Setenv("MYSQL_ENDPOINT", sharedTiDBCluster.Endpoint)
		os.Setenv("MYSQL_USERNAME", sharedTiDBCluster.Username)
		os.Setenv("MYSQL_PASSWORD", sharedTiDBCluster.Password)

		// Run all tests
		code := m.Run()

		// Cleanup shared TiDB cluster after all tests complete
		cleanupSharedTiDBCluster()

		// Exit with test result code
		os.Exit(code)
	}

	// Default to MySQL 8.0, but allow override via DOCKER_IMAGE env var
	mysqlImage := os.Getenv("DOCKER_IMAGE")
	if mysqlImage == "" {
		mysqlImage = "mysql:8.0"
	}

	// Start shared container before running tests
	var err error
	sharedContainerMtx.Lock()
	sharedContainer, err = startSharedMySQLContainer(mysqlImage)
	sharedContainerMtx.Unlock()

	if err != nil {
		// If container startup fails, exit with error
		os.Stderr.WriteString(fmt.Sprintf("Failed to start shared MySQL container: %v\n", err))
		os.Exit(1)
	}

	// Set up environment variables for the shared container
	os.Setenv("MYSQL_ENDPOINT", sharedContainer.Endpoint)
	os.Setenv("MYSQL_USERNAME", sharedContainer.Username)
	os.Setenv("MYSQL_PASSWORD", sharedContainer.Password)

	// Run all tests
	code := m.Run()

	// Cleanup shared container after all tests complete
	cleanupSharedContainer()

	// Exit with test result code
	os.Exit(code)
}
