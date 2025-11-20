//go:build testcontainers
// +build testcontainers

package mysql

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccDataSourceTables_WithTestcontainers tests the mysql_tables data source
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain
func TestAccDataSourceTables_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	// Run the same test logic as the original test
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTablesConfigBasic("mysql", "%"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_tables.test", "database", "mysql"),
					resource.TestCheckResourceAttr("data.mysql_tables.test", "pattern", "%"),
					testAccTablesCount("data.mysql_tables.test", "tables.#", func(rn string, tableCount int) error {
						if tableCount < 1 {
							return fmt.Errorf("%s: tables not found", rn)
						}
						return nil
					}),
				),
			},
			{
				Config: testAccTablesConfigBasic("mysql", "__table_does_not_exist__"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.mysql_tables.test", "database", "mysql"),
					resource.TestCheckResourceAttr("data.mysql_tables.test", "pattern", "__table_does_not_exist__"),
					testAccTablesCount("data.mysql_tables.test", "tables.#", func(rn string, tableCount int) error {
						if tableCount > 0 {
							return fmt.Errorf("%s: unexpected table found", rn)
						}
						return nil
					}),
				),
			},
		},
	})
}
