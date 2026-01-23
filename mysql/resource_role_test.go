package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
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
	rows, err := db.Query(fmt.Sprintf("SHOW GRANTS FOR %s", quoteIdentifier(roleName)))
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

func testAccRoleConfigDifferent(roleName string) string {
	return fmt.Sprintf(`
resource "mysql_role" "different" {
  name = "%s"
}
`, roleName)
}

func testAccRoleConfigMultiple(roleName1, roleName2 string) string {
	return fmt.Sprintf(`
resource "mysql_role" "test1" {
  name = "%s"
}

resource "mysql_role" "test2" {
  name = "%s"
}
`, roleName1, roleName2)
}

func TestAccRole_importBasic(t *testing.T) {
	roleName := "tf-test-role-import-basic"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importSpecialChars(t *testing.T) {
	roleName := "tf-test-role@#$%"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importAndPlan(t *testing.T) {
	roleName := "tf-test-role-import-plan"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
			{
				Config:             testAccRoleConfigBasic(roleName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccRole_importAndDestroy(t *testing.T) {
	roleName := "tf-test-role-import-destroy"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importWithDifferentResourceName(t *testing.T) {
	roleName := "tf-test-role-diff-resource"
	resourceName := "mysql_role.different"

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
				Config: testAccRoleConfigDifferent(roleName),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importMultipleRoles(t *testing.T) {
	roleName1 := "tf-test-role-multi1"
	roleName2 := "tf-test-role-multi2"
	resourceName1 := "mysql_role.test1"
	resourceName2 := "mysql_role.test2"

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
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccRoleCheckDestroy(roleName1),
			testAccRoleCheckDestroy(roleName2),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfigMultiple(roleName1, roleName2),
			},
			{
				ResourceName:      resourceName1,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName1,
			},
			{
				ResourceName:      resourceName2,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName2,
			},
		},
	})
}

func TestAccRole_importNonExistent(t *testing.T) {
	roleName := "tf-test-role-nonexistent"

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
		Steps: []resource.TestStep{
			{
				Config: " ",
			},
			{
				ResourceName:  "mysql_role.test",
				ImportState:   true,
				ImportStateId: roleName,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 0 {
						return fmt.Errorf("expected no states, got %d", len(states))
					}
					return nil
				},
			},
		},
	})
}

func TestAccRole_importWithGrants(t *testing.T) {
	roleName := "tf-test-role-with-grants"
	resourceName := "mysql_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipMariaDB(t)
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
				Config: testAccRoleConfigWithGrants(roleName),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importAndUpdate(t *testing.T) {
	roleName1 := "tf-test-role-update1"
	roleName2 := "tf-test-role-update2"
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
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccRoleCheckDestroy(roleName1),
			testAccRoleCheckDestroy(roleName2),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfigBasic(roleName1),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName1,
			},
			{
				Config: testAccRoleConfigBasic(roleName2),
				Check: resource.ComposeTestCheckFunc(
					testAccRoleExists(roleName2),
					resource.TestCheckResourceAttr(resourceName, "name", roleName2),
				),
			},
		},
	})
}

func TestAccRole_importConcurrency(t *testing.T) {
	roleName1 := "tf-test-role-conc1"
	roleName2 := "tf-test-role-conc2"
	resourceName1 := "mysql_role.test1"
	resourceName2 := "mysql_role.test2"

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
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccRoleCheckDestroy(roleName1),
			testAccRoleCheckDestroy(roleName2),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfigMultiple(roleName1, roleName2),
			},
			{
				ResourceName:      resourceName1,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName1,
			},
			{
				ResourceName:      resourceName2,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName2,
			},
		},
	})
}

func TestAccRole_importSingleQuote(t *testing.T) {
	roleName := "tf-test-role'quote"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importBackslash(t *testing.T) {
	roleName := "tf-test-role\\\\backslash"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importDoubleQuote(t *testing.T) {
	roleName := "tf-test-role\\\"quote-test"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importUnicode(t *testing.T) {
	roleName := "tf-test-role-unicode-测试"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importReservedWord(t *testing.T) {
	roleName := "SELECT"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importLongName(t *testing.T) {
	roleName := "tf-test-role-long-" + strings.Repeat("a", 46)
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func TestAccRole_importMultipleSpecialChars(t *testing.T) {
	roleName := "tf-test-role@#$%^&*()"
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
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     roleName,
			},
		},
	})
}

func testAccRoleConfigWithGrants(roleName string) string {
	return fmt.Sprintf(`
resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  user       = "%s"
  host       = "%%"
  database   = "*"
  table      = "*"
  privileges = ["SELECT"]
}
`, roleName, roleName)
}
