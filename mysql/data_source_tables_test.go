package mysql

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceTables(t *testing.T) {
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

func testAccTablesCount(rn string, key string, check func(string, int) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]

		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		value, ok := rs.Primary.Attributes[key]

		if !ok {
			return fmt.Errorf("%s: attribute '%s' not found", rn, key)
		}

		tableCount, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		return check(rn, tableCount)
	}
}

func testAccTablesConfigBasic(database string, pattern string) string {
	return fmt.Sprintf(`
data "mysql_tables" "test" {
		database = "%s"
		pattern = "%s"
}`, database, pattern)
}
