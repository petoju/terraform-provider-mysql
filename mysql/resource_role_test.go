package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccRole_basic(t *testing.T) {
	roleName := "tf-test-role"
	resourceName := "mysql_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			ctx := context.Background()
			db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
			if err != nil {
				return
			}

			requiredVersion, _ := version.NewVersion("8.0.0")
			currentVersion, err := serverVersion(db)
			if err != nil {
				return
			}

			if currentVersion.LessThan(requiredVersion) {
				t.Skip("Roles require MySQL 8+")
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccRoleCheckDestroy(roleName),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfigBasic(roleName),
				Check: resource.ComposeTestCheckFunc(
					testAccRoleExists(roleName),
					resource.TestCheckResourceAttr(resourceName, "name", roleName),
				),
			},
		},
	})
}

func testAccRoleExists(roleName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		count, err := testAccGetRoleGrantCount(roleName, db)

		if err != nil {
			return err
		}

		if count > 0 {
			return nil
		}

		return fmt.Errorf("no grants found for role %s", roleName)
	}
}

func testAccGetRoleGrantCount(roleName string, db *sql.DB) (int, error) {
	rows, err := db.Query(fmt.Sprintf("SHOW GRANTS FOR '%s'", roleName))
	if err != nil {
		return 0, err
	}

	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	return count, nil
}

func testAccRoleCheckDestroy(roleName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		count, err := testAccGetRoleGrantCount(roleName, db)
		if count > 0 {
			return fmt.Errorf("role %s still has grants/exists", roleName)
		}

		return nil
	}
}

func testAccRoleConfigBasic(roleName string) string {
	return fmt.Sprintf(`
resource "mysql_role" "test" {
  name = "%s"
}
`, roleName)
}
