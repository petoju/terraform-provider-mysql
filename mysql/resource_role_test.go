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
		Providers:    testAccProviders,
		CheckDestroy: testAccRoleCheckDestroy(roleName),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfig_basic(roleName),
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

		return fmt.Errorf("No grants found for role %s", roleName)
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
			return fmt.Errorf("Role %s still has grants/exists", roleName)
		}

		return nil
	}
}

func testAccRoleConfig_basic(roleName string) string {
	return fmt.Sprintf(`
resource "mysql_role" "test" {
  name = "%s"
}
`, roleName)
}

func TestAccGrantOnProcedure(t *testing.T) {
	procedureName := "test_procedure"
	userName := "test_user"
	hostName := "%"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigProcedure(procedureName, userName, hostName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProcedureGrant("mysql_grant.test_procedure", userName, hostName, procedureName, true),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "database", fmt.Sprintf("PROCEDURE %s", procedureName)),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "table", ""), // Ensure table attribute is empty
				),
			},
		},
	})
}

func testAccGrantConfigProcedure(procedureName, userName, hostName string) string {
	return fmt.Sprintf(`
resource "mysql_grant" "test_procedure" {
    user       = "%s"
    host       = "%s"
    privileges = ["EXECUTE"]
    database   = "PROCEDURE %s"
}
`, userName, hostName, procedureName)
}

func testAccCheckProcedureGrant(resourceName, userName, hostName, procedureName string, expected bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Obtain the database connection from the Terraform provider
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		// Query the database to check if the grant exists
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT * FROM information_schema.routine_privileges WHERE grantee = '%s@%s' AND routine_name = '%s' AND privilege_type = 'EXECUTE')", userName, hostName, procedureName)
		err = db.QueryRow(query).Scan(&exists)
		if err != nil {
			return err
		}

		// Compare the result with the expected outcome
		if exists != expected {
			return fmt.Errorf("Grant for procedure %s does not match expected state: %v", procedureName, expected)
		}

		return nil
	}
}
