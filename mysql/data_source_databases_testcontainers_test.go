//go:build testcontainers
// +build testcontainers

package mysql

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccDataSourceDatabases_WithTestcontainers is a proof of concept test
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain
func TestAccDataSourceDatabases_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	// Run the same test logic as the original test
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDatabasesConfigBasic("%"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_databases.test", "pattern", "%"),
					testAccDatabasesCount("data.mysql_databases.test", "databases.#", func(rn string, databaseCount int) error {
						if databaseCount < 1 {
							return fmt.Errorf("%s: databases not found", rn)
						}
						return nil
					}),
				),
			},
			{
				Config: testAccDatabasesConfigBasic("__database_does_not_exist__"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_databases.test", "pattern", "__database_does_not_exist__"),
					testAccDatabasesCount("data.mysql_databases.test", "databases.#", func(rn string, databaseCount int) error {
						if databaseCount > 0 {
							return fmt.Errorf("%s: unexpected database found", rn)
						}
						return nil
					}),
				),
			},
		},
	})
}
