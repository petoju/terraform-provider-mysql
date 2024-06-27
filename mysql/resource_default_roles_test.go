package mysql

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDefaultRoles_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotMySQL8(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDefaultRolesCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDefaultRolesBasic,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test", "role1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.0", "role1"),
				),
			},
			{
				Config: testAccDefaultRolesMultiple,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test", "role1", "role2"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.0", "role1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.1", "role2"),
				),
			},
			{
				Config: testAccDefaultRolesNone,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "0"),
				),
			},
			{
				Config:            testAccDefaultRolesBasic,
				ResourceName:      "mysql_default_roles.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v", "jdoe", "%"),
			},
			{
				Config:            testAccDefaultRolesMultiple,
				ResourceName:      "mysql_default_roles.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v", "jdoe", "%"),
			},
		},
	})
}

func testAccDefaultRoles(rn string, roles ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("default roles id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		stmtSQL := fmt.Sprintf("SELECT default_role_user from mysql.default_roles where CONCAT(user, '@', host) = '%s'", rs.Primary.ID)
		log.Println("[DEBUG] Executing statement:", stmtSQL)
		rows, err := db.Query(stmtSQL)
		if err != nil {
			return fmt.Errorf("error reading user default roles: %w", err)
		}
		defer rows.Close()

		dbRoles := make([]string, 0)

		for rows.Next() {
			var role string
			err := rows.Scan(&role)
			if err != nil {
				return fmt.Errorf("error reading user default roles: %w", err)
			}
			dbRoles = append(dbRoles, role)
		}

		if len(dbRoles) != len(roles) {
			return fmt.Errorf("expected %d rows reading user default roles but got %d", len(roles), len(dbRoles))
		}

		exists := make(map[string]bool)
		for _, role := range dbRoles {
			exists[role] = true
		}
		for _, role := range roles {
			if !exists[role] {
				return fmt.Errorf("expected role %s in user default roles but it was not found", role)
			}
		}

		return nil
	}
}

func testAccDefaultRolesCheckDestroy(s *terraform.State) error {
	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mysql_default_roles" {
			continue
		}

		stmtSQL := fmt.Sprintf("SELECT count(*) FROM mysql.default_roles WHERE CONCAT(user, '@', host) = '%s'", rs.Primary.ID)
		log.Println("[DEBUG] Executing statement:", stmtSQL)
		var count int
		err := db.QueryRow(stmtSQL).Scan(&count)
		if err != nil {
			return fmt.Errorf("error issuing query: %w", err)
		}
		if count > 0 {
			return fmt.Errorf("default roles still exist after destroy")
		}
	}
	return nil
}

const testAccDefaultRolesBasic = `
resource "mysql_role" "role1" {
	name = "role1"
}

resource "mysql_user" "test" {
	user = "jdoe"
	host = "%"
}

resource "mysql_grant" "test" {
	user     = mysql_user.test.user
	host     = mysql_user.test.host
	database = ""
	roles    = [mysql_role.role1.name]
}

resource "mysql_default_roles" "test" {
	user = mysql_user.test.user
	host = mysql_user.test.host
	roles = mysql_grant.test.roles
}
`

const testAccDefaultRolesMultiple = `
resource "mysql_role" "role1" {
	name = "role1"
}

resource "mysql_role" "role2" {
	name = "role2"
}

resource "mysql_user" "test" {
	user = "jdoe"
	host = "%"
}

resource "mysql_grant" "test" {
	user     = mysql_user.test.user
	host     = mysql_user.test.host
	database = ""
	roles    = [mysql_role.role1.name, mysql_role.role2.name]
}

resource "mysql_default_roles" "test" {
	user = mysql_user.test.user
	host = mysql_user.test.host
	roles = mysql_grant.test.roles
}
`

const testAccDefaultRolesNone = `
resource "mysql_user" "test" {
	user = "jdoe"
	host = "%"
}

resource "mysql_default_roles" "test" {
	user = mysql_user.test.user
	host = mysql_user.test.host
	roles = []
}
`
