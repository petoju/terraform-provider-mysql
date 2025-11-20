# Spike: Replacing Makefile + Docker with Testcontainers

## Current State

### Problems with Current Approach

1. **Complex Makefile**: 200+ lines of shell scripting with port calculations, container management, and cleanup logic
2. **Fragile Port Management**: Complex port calculation logic (`34$(tr -d '.')`) that breaks with longer version numbers
3. **Manual Container Lifecycle**: Containers must be manually started, waited for, and cleaned up
4. **TiDB Complexity**: Requires custom bash script (`tidb-test-cluster.sh`) with 200+ lines for multi-container orchestration
5. **No Parallel Execution**: Containers started sequentially, tests run sequentially
6. **Environment Variable Pollution**: Tests rely on global environment variables
7. **Hard to Debug**: Container failures require manual log inspection
8. **CI/CD Complexity**: GitHub Actions workflow has to coordinate Makefile targets with Docker

### Current Test Flow

```
Makefile target → Docker run → Wait loop → Set env vars → Run tests → Cleanup
```

## Testcontainers Solution

[Testcontainers](https://golang.testcontainers.org/) is a Go library that provides lightweight, throwaway instances of Docker containers for integration testing.

### Benefits

1. **Pure Go**: No shell scripts, no Makefile complexity
2. **Automatic Lifecycle**: Containers start/stop automatically with test lifecycle
3. **Built-in Waiting**: Automatic readiness checks (no manual wait loops)
4. **Parallel Execution**: Go's testing framework handles parallel test execution
5. **Isolated Tests**: Each test can have its own container instance
6. **Better Error Messages**: Container logs automatically captured on failure
7. **Type Safety**: Compile-time checks instead of runtime shell errors
8. **Simpler CI/CD**: Just run `go test` - no Makefile coordination needed

## Proof of Concept

### Example: Simple MySQL Test

```go
package mysql

import (
    "context"
    "os"
    "testing"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    tcMySQL "github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestAccDatabase_WithTestcontainers(t *testing.T) {
    ctx := context.Background()
    
    // Start MySQL container
    mysqlContainer, err := tcMySQL.RunContainer(ctx,
        testcontainers.WithImage("mysql:8.0"),
        tcMySQL.WithDatabase("testdb"),
        tcMySQL.WithUsername("root"),
        tcMySQL.WithPassword(""),
        testcontainers.WithWaitStrategy(
            wait.ForLog("ready for connections").
                WithOccurrence(2).
                WithStartupTimeout(120*time.Second),
        ),
    )
    if err != nil {
        t.Fatalf("Failed to start container: %v", err)
    }
    defer func() {
        if err := mysqlContainer.Terminate(ctx); err != nil {
            t.Fatalf("Failed to terminate container: %v", err)
        }
    }()
    
    // Get connection details
    endpoint, err := mysqlContainer.ConnectionString(ctx)
    if err != nil {
        t.Fatalf("Failed to get connection string: %v", err)
    }
    
    // Set environment variables for test
    os.Setenv("MYSQL_ENDPOINT", endpoint)
    os.Setenv("MYSQL_USERNAME", "root")
    os.Setenv("MYSQL_PASSWORD", "")
    defer func() {
        os.Unsetenv("MYSQL_ENDPOINT")
        os.Unsetenv("MYSQL_USERNAME")
        os.Unsetenv("MYSQL_PASSWORD")
    }()
    
    // Run existing test logic
    resource.Test(t, resource.TestCase{
        PreCheck:          func() { testAccPreCheck(t) },
        ProviderFactories: testAccProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testAccDatabaseConfig("testdb"),
                Check: resource.ComposeTestCheckFunc(
                    testAccDatabaseExists("mysql_database.test"),
                ),
            },
        },
    })
}
```

### Example: Test Helper Function

```go
// testcontainers_helper.go

package mysql

import (
    "context"
    "os"
    "testing"
    "time"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
    tcMySQL "github.com/testcontainers/testcontainers-go/modules/mysql"
)

type MySQLTestContainer struct {
    Container testcontainers.Container
    Endpoint  string
    Username  string
    Password  string
}

func startMySQLContainer(ctx context.Context, t *testing.T, image string) *MySQLTestContainer {
    container, err := tcMySQL.RunContainer(ctx,
        testcontainers.WithImage(image),
        tcMySQL.WithDatabase("testdb"),
        tcMySQL.WithUsername("root"),
        tcMySQL.WithPassword(""),
        testcontainers.WithWaitStrategy(
            wait.ForLog("ready for connections").
                WithOccurrence(2).
                WithStartupTimeout(120*time.Second),
        ),
    )
    if err != nil {
        t.Fatalf("Failed to start MySQL container: %v", err)
    }
    
    endpoint, err := container.ConnectionString(ctx)
    if err != nil {
        t.Fatalf("Failed to get connection string: %v", err)
    }
    
    return &MySQLTestContainer{
        Container: container,
        Endpoint:  endpoint,
        Username:  "root",
        Password:  "",
    }
}

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

// Usage in tests
func TestAccDatabase_Simple(t *testing.T) {
    ctx := context.Background()
    container := startMySQLContainer(ctx, t, "mysql:8.0")
    defer container.SetupTestEnv(t)()
    
    resource.Test(t, resource.TestCase{
        PreCheck:          func() { testAccPreCheck(t) },
        ProviderFactories: testAccProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testAccDatabaseConfig("testdb"),
                Check: resource.ComposeTestCheckFunc(
                    testAccDatabaseExists("mysql_database.test"),
                ),
            },
        },
    })
}
```

### Example: Percona and MariaDB Support

```go
func startPerconaContainer(ctx context.Context, t *testing.T, version string) *MySQLTestContainer {
    image := fmt.Sprintf("percona:%s", version)
    // Percona uses same MySQL protocol, can use MySQL module
    return startMySQLContainer(ctx, t, image)
}

func startMariaDBContainer(ctx context.Context, t *testing.T, version string) *MySQLTestContainer {
    image := fmt.Sprintf("mariadb:%s", version)
    // MariaDB also uses MySQL protocol
    return startMySQLContainer(ctx, t, image)
}
```

### Example: TiDB Multi-Container Setup

```go
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/compose"
)

func startTiDBCluster(ctx context.Context, t *testing.T, version string) *MySQLTestContainer {
    // Option 1: Use Docker Compose module
    composeFile := fmt.Sprintf(`
version: '3'
services:
  pd:
    image: pingcap/pd:v%s
    command: --name=pd --data-dir=/data --client-urls=http://0.0.0.0:2379 --peer-urls=http://0.0.0.0:2380
  tikv:
    image: pingcap/tikv:v%s
    depends_on: [pd]
    command: --addr=0.0.0.0:20160 --advertise-addr=tikv:20160 --pd=pd:2379
  tidb:
    image: pingcap/tidb:v%s
    depends_on: [tikv]
    ports:
      - "4000"
    command: --store=tikv --path=pd:2379
`, version, version, version)
    
    composeContainer, err := compose.NewDockerCompose(composeFile)
    if err != nil {
        t.Fatalf("Failed to create compose: %v", err)
    }
    
    err = composeContainer.Up(ctx, compose.Wait(true))
    if err != nil {
        t.Fatalf("Failed to start TiDB cluster: %v", err)
    }
    
    // Get TiDB port
    tidbPort, err := composeContainer.ServicePort(ctx, "tidb", 4000)
    if err != nil {
        t.Fatalf("Failed to get TiDB port: %v", err)
    }
    
    return &MySQLTestContainer{
        Endpoint: fmt.Sprintf("127.0.0.1:%s", tidbPort.Port()),
        Username: "root",
        Password: "",
    }
}

// Option 2: Manual multi-container setup
func startTiDBClusterManual(ctx context.Context, t *testing.T, version string) *MySQLTestContainer {
    networkName := "tidb-test-network"
    network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
        NetworkRequest: testcontainers.NetworkRequest{
            Name: networkName,
        },
    })
    if err != nil {
        t.Fatalf("Failed to create network: %v", err)
    }
    
    // Start PD
    pdContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: fmt.Sprintf("pingcap/pd:v%s", version),
            Networks: []string{networkName},
            Cmd: []string{
                "--name=pd",
                "--data-dir=/data",
                "--client-urls=http://0.0.0.0:2379",
                "--peer-urls=http://0.0.0.0:2380",
            },
        },
    })
    if err != nil {
        t.Fatalf("Failed to start PD: %v", err)
    }
    
    // Start TiKV
    tikvContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: fmt.Sprintf("pingcap/tikv:v%s", version),
            Networks: []string{networkName},
            Cmd: []string{
                "--addr=0.0.0.0:20160",
                "--advertise-addr=tikv:20160",
                "--pd=pd:2379",
            },
        },
    })
    if err != nil {
        t.Fatalf("Failed to start TiKV: %v", err)
    }
    
    // Start TiDB
    tidbContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image: fmt.Sprintf("pingcap/tidb:v%s", version),
            ExposedPorts: []string{"4000/tcp"},
            Networks: []string{networkName},
            Cmd: []string{
                "--store=tikv",
                "--path=pd:2379",
            },
            WaitingFor: wait.ForLog("server is running MySQL protocol").WithStartupTimeout(240 * time.Second),
        },
    })
    if err != nil {
        t.Fatalf("Failed to start TiDB: %v", err)
    }
    
    tidbPort, err := tidbContainer.MappedPort(ctx, "4000")
    if err != nil {
        t.Fatalf("Failed to get TiDB port: %v", err)
    }
    
    return &MySQLTestContainer{
        Endpoint: fmt.Sprintf("127.0.0.1:%s", tidbPort.Port()),
        Username: "root",
        Password: "",
    }
}
```

### Example: Table-Driven Tests for Multiple Versions

```go
func TestAccDatabase_MultipleVersions(t *testing.T) {
    versions := []struct {
        name  string
        image string
    }{
        {"MySQL5.6", "mysql:5.6"},
        {"MySQL5.7", "mysql:5.7"},
        {"MySQL8.0", "mysql:8.0"},
        {"Percona5.7", "percona:5.7"},
        {"Percona8.0", "percona:8.0"},
        {"MariaDB10.3", "mariadb:10.3"},
        {"MariaDB10.8", "mariadb:10.8"},
        {"MariaDB10.10", "mariadb:10.10"},
    }
    
    for _, v := range versions {
        t.Run(v.name, func(t *testing.T) {
            ctx := context.Background()
            container := startMySQLContainer(ctx, t, v.image)
            defer container.SetupTestEnv(t)()
            
            resource.Test(t, resource.TestCase{
                PreCheck:          func() { testAccPreCheck(t) },
                ProviderFactories: testAccProviderFactories,
                Steps: []resource.TestStep{
                    {
                        Config: testAccDatabaseConfig("testdb"),
                        Check: resource.ComposeTestCheckFunc(
                            testAccDatabaseExists("mysql_database.test"),
                        ),
                    },
                },
            })
        })
    }
}
```

## Migration Strategy

### Phase 1: Proof of Concept (Current)
- Create spike document ✅
- Implement helper functions for MySQL/Percona/MariaDB
- Convert one test file as proof of concept
- Measure performance and reliability

### Phase 2: Gradual Migration
- Convert tests file-by-file
- Keep Makefile targets as fallback during migration
- Update CI/CD to support both approaches

### Phase 3: Complete Migration
- Remove Makefile targets
- Remove shell scripts (`tidb-test-cluster.sh`)
- Simplify GitHub Actions workflow
- Update documentation

## Benefits Analysis

### Development Experience
- ✅ **Simpler**: No Makefile, no shell scripts
- ✅ **Type Safe**: Compile-time errors instead of runtime shell errors
- ✅ **Better IDE Support**: Go tooling works out of the box
- ✅ **Easier Debugging**: Container logs automatically captured

### Testing
- ✅ **Parallel Execution**: Go's testing framework handles this
- ✅ **Isolation**: Each test can have its own container
- ✅ **Reliability**: Automatic retries and better error handling
- ✅ **Port Management**: No manual port calculation needed

### CI/CD
- ✅ **Simpler Workflow**: Just `go test` - no Makefile coordination
- ✅ **Better Caching**: Testcontainers can reuse containers
- ✅ **Faster**: Parallel test execution
- ✅ **More Reliable**: Better error messages and retry logic

## Trade-offs and Considerations

### Challenges

1. **TiDB Complexity**: Multi-container setup still complex, but better than shell scripts
2. **Learning Curve**: Team needs to learn Testcontainers API
3. **Docker Dependency**: Still requires Docker (same as current approach)
4. **Migration Effort**: Need to convert all tests

### Potential Issues

1. **Container Startup Time**: May be slower than optimized Makefile approach
   - **Mitigation**: Testcontainers has built-in caching and reuse
   
2. **Resource Usage**: Multiple containers running in parallel
   - **Mitigation**: Containers are lightweight, can limit parallelism
   
3. **Network Issues**: Docker networking complexity
   - **Mitigation**: Testcontainers handles this automatically

## Implementation Plan

### Step 1: Add Dependencies

```bash
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/mysql
go get github.com/testcontainers/testcontainers-go/modules/compose
```

### Step 2: Create Helper Package

Create `mysql/testcontainers_helper.go` with:
- `startMySQLContainer()` - MySQL/Percona/MariaDB
- `startTiDBCluster()` - TiDB multi-container setup
- `MySQLTestContainer` struct with cleanup logic

### Step 3: Convert One Test File

Start with `data_source_databases_test.go` as proof of concept:
- Replace `testAccPreCheck` to use Testcontainers
- Verify it works locally
- Measure performance

### Step 4: Update CI/CD

Update `.github/workflows/main.yml`:
- Remove Makefile targets
- Use `go test -tags=acceptance` with build tags
- Run tests in parallel using Go's built-in parallelism

### Step 5: Gradual Migration

Convert tests file-by-file:
1. `data_source_databases_test.go`
2. `resource_database_test.go`
3. `resource_user_test.go`
4. ... (continue with remaining files)

## Performance Comparison

### Current Approach
- Sequential container startup: ~10-30s per version
- Sequential test execution: ~60-90s per test suite
- Total for 13 versions: ~15-20 minutes

### Testcontainers Approach (Estimated)
- Parallel container startup: ~10-30s (same, but parallel)
- Parallel test execution: ~60-90s (same, but parallel)
- Total for 13 versions: ~2-3 minutes (with parallelism)

## Next Steps

1. **Review this spike** with team
2. **Implement proof of concept** for one test file
3. **Measure performance** and compare with current approach
4. **Decide on migration strategy** (gradual vs. all-at-once)
5. **Create migration tickets** if approved

## References

- [Testcontainers Go Documentation](https://golang.testcontainers.org/)
- [Testcontainers MySQL Module](https://golang.testcontainers.org/modules/mysql/)
- [Testcontainers Compose Module](https://golang.testcontainers.org/modules/compose/)
- [Example: Terraform Provider Testing](https://github.com/testcontainers/testcontainers-go/tree/main/examples)
