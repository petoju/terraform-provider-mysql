# Testcontainers Proof of Concept

This directory contains a proof of concept implementation using Testcontainers instead of the Makefile + Docker approach.

## What's Implemented

1. **`mysql/testcontainers_helper.go`**: Helper functions for starting MySQL containers
2. **`mysql/data_source_databases_testcontainers_test.go`**: Example test using Testcontainers

## How to Use

### Prerequisites

- Docker or Podman installed
- Go 1.21+ (project requirement)

### Running Tests

```bash
# Run the Testcontainers-based test
go test -tags=testcontainers -v ./mysql/... -run TestAccDataSourceDatabases_WithTestcontainers

# Run all tests (both old and new)
go test -tags=testcontainers -v ./mysql/...
```

### With Podman

Testcontainers automatically detects Podman. To use Podman explicitly:

```bash
# Set Podman socket (if needed)
export CONTAINER_HOST=unix://$HOME/.local/share/containers/podman/machine/podman-machine-default/podman.sock

# Run tests
go test -tags=testcontainers -v ./mysql/... -run TestAccDataSourceDatabases_WithTestcontainers
```

## How It Works

1. **Build Tag**: Code is gated behind `// +build testcontainers` tag
   - Old tests continue to work without Testcontainers
   - New tests only run when `-tags=testcontainers` is used

2. **Container Lifecycle**: 
   - Container starts automatically when test begins
   - Environment variables are set for the test
   - Container terminates automatically when test completes

3. **Compatibility**: 
   - Works with Docker (default)
   - Works with Podman (automatic detection)
   - Uses GenericContainer for Go 1.21 compatibility

## Example Test

```go
func TestAccDataSourceDatabases_WithTestcontainers(t *testing.T) {
    ctx := context.Background()
    
    // Start MySQL container
    container := startMySQLContainer(ctx, t, "mysql:8.0")
    defer container.SetupTestEnv(t)()
    
    // Run tests (same as before)
    resource.Test(t, resource.TestCase{
        PreCheck:          func() { testAccPreCheck(t) },
        ProviderFactories: testAccProviderFactories,
        Steps: []resource.TestStep{
            // ... test steps
        },
    })
}
```

## Benefits Over Makefile Approach

1. **No Shell Scripts**: Pure Go code
2. **Automatic Cleanup**: Containers terminate automatically
3. **Better Error Messages**: Container logs captured on failure
4. **Parallel Execution**: Go's testing framework handles parallelism
5. **Podman Support**: Works without configuration changes

## Next Steps

1. **Test Locally**: Verify it works with your Docker/Podman setup
2. **Measure Performance**: Compare startup time vs. Makefile approach
3. **Convert More Tests**: Gradually migrate other test files
4. **CI/CD Integration**: Update GitHub Actions to use Testcontainers

## Troubleshooting

### Compiler Warnings

You may see warnings like:
```
warning: variable length array folded to constant array as an extension [-Wgnu-folding-constant]
```

These are harmless warnings from the `go-m1cpu` dependency (used by testcontainers-go). They don't affect functionality. To suppress them:

```bash
export CGO_CFLAGS="-Wno-gnu-folding-constant"
go test -tags=testcontainers -v ./mysql/...
```

Or use the provided script:
```bash
source .testcontainers-build-flags.sh
go test -tags=testcontainers -v ./mysql/...
```

### Container Won't Start

- Check Docker/Podman is running: `docker ps` or `podman ps`
- Check image exists: `docker images mysql:8.0`
- Increase timeout in `testcontainers_helper.go` if needed

### Port Conflicts

- Testcontainers automatically assigns random ports
- No manual port calculation needed (unlike Makefile approach)

### Podman Issues

- Ensure Podman socket is accessible
- Check `CONTAINER_HOST` environment variable if needed
- Testcontainers should auto-detect Podman
