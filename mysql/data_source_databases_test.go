package mysql

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDataSourceDatabases(t *testing.T) {
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

func testAccDatabasesCount(rn string, key string, check func(string, int) error) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]

		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		value, ok := rs.Primary.Attributes[key]

		if !ok {
			return fmt.Errorf("%s: attribute '%s' not found", rn, key)
		}

		databaseCount, err := strconv.Atoi(value)

		if err != nil {
			return err
		}

		return check(rn, databaseCount)
	}
}

func testAccDatabasesConfigBasic(pattern string) string {
	return fmt.Sprintf(`
data "mysql_databases" "test" {
		pattern = "%s"
}`, pattern)
}
