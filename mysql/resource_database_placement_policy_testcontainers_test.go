//go:build testcontainers
// +build testcontainers

package mysql

// Note: TestAccDatabase_placementPolicyChange_WithTestcontainers is skipped
// because placement policies are TiDB-specific and require a TiDB cluster setup,
// which is more complex than a simple MySQL container.
// This test would need special TiDB container orchestration.
