package mysql

import (
	"context"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

func TestAccGrant(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPreCheckSkipRds(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
				),
			},
		},
	})
}

func TestAccGrantWithGrantOption(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
			{
				Config: testAccGrantConfigBasicWithGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
		},
	})
}

func TestAccBroken(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
			{
				Config:      testAccGrantConfigBroken(dbName),
				ExpectError: regexp.MustCompile("already has"),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
		},
	})
}

func TestAccDifferentHosts(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigExtraHost(dbName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test_all", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "host", "%"),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfigExtraHost(dbName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "10.1.2.3"),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "host", "%"),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "table", "*"),
				),
			},
		},
	})
}

func TestAccGrantComplex(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheckSkipTiDB(t); testAccPreCheckSkipRds(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT (c1, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1)", "INSERT(c3, c4)", "REFERENCES(c5)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "INSERT (c3,c4)", true),
					testAccPrivilege("mysql_grant.test", "SELECT (c1)", true),
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", false),
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1)", "INSERT(c4, c3, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL PRIVILEGES"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1, c2)", "INSERT(c5)", "REFERENCES(c1)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", false),
					testAccPrivilege("mysql_grant.test", "DROP", true),
					testAccPrivilege("mysql_grant.test", "SELECT(c1,c2)", true),
					testAccPrivilege("mysql_grant.test", "INSERT(c5)", true),
					testAccPrivilege("mysql_grant.test", "REFERENCES(c1)", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
		},
	})
}

func TestAccGrantComplexMySQL8(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipTiDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName),
				),
			},
			{
				Config: testAccGrantConfigWithDynamicMySQL8(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SHOW DATABASES", true),
					testAccPrivilege("mysql_grant.test", "CONNECTION_ADMIN", true),
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
		},
	})
}

func TestAccGrant_role(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigRole(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
			{
				Config: testAccGrantConfigRoleWithGrantOption(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
			{
				Config: testAccGrantConfigRole(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
		},
	})
}

func TestAccGrant_roleToUser(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigRoleToUser(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "roles.#", "1"),
				),
			},
		},
	})
}

func TestAccGrant_complexRoleGrants(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigComplexRoleGrants(dbName),
			},
		},
	})
}

func prepareTable(dbname string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE `%s`.`tbl`(c1 INT, c2 INT, c3 INT,c4 INT,c5 INT);", dbname)); err != nil {
			return fmt.Errorf("error reading grant: %s", err)
		}
		return nil
	}
}

// Test privilege - one can condition it exists or that it doesn't exist.
func testAccPrivilege(rn string, privilege string, expectExists bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("grant id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		id := strings.Split(rs.Primary.ID, ":")

		var userOrRole string
		if strings.Contains(id[0], "@") {
			parts := strings.Split(id[0], "@")
			userOrRole = fmt.Sprintf("'%s'@'%s'", parts[0], parts[1])
		} else {
			userOrRole = fmt.Sprintf("'%s'", id[0])
		}

		grants, err := showUserGrants(context.Background(), db, userOrRole)
		if err != nil {
			return err
		}

		privilegeNorm := normalizePerms([]string{privilege})[0]

		haveGrant := false

	Outer:
		for _, grant := range grants {
			privs := normalizePerms(grant.Privileges)
			for _, priv := range privs {
				if priv == privilegeNorm {
					haveGrant = true
					break Outer
				}
			}
		}

		if expectExists != haveGrant {
			if haveGrant {
				return fmt.Errorf("grant %s found but it was not requested for %s", privilege, userOrRole)
			} else {
				return fmt.Errorf("grant %s not found for %s", privilege, userOrRole)
			}
		}

		// We match expectations.
		return nil
	}
}

func testAccGrantCheckDestroy(s *terraform.State) error {
	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mysql_grant" {
			continue
		}

		id := strings.Split(rs.Primary.ID, ":")

		var userOrRole string
		if strings.Contains(id[0], "@") {
			parts := strings.Split(id[0], "@")
			userOrRole = fmt.Sprintf("'%s'@'%s'", parts[0], parts[1])
		} else {
			userOrRole = fmt.Sprintf("'%s'", id[0])
		}

		stmtSQL := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole)
		log.Printf("[DEBUG] SQL: %s", stmtSQL)
		rows, err := db.Query(stmtSQL)
		if err != nil {
			if isNonExistingGrant(err) {
				return nil
			}

			return fmt.Errorf("error reading grant: %s", err)
		}

		if rows.Next() {
			return fmt.Errorf("grant still exists for: %s", userOrRole)
		}
		rows.Close()
	}
	return nil
}

func testAccGrantConfigNoGrant(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_user" "test_global" {
  user     = "jdoe-%s"
  host     = "%%"
}

`, dbName, dbName, dbName)
}

func testAccGrantConfigWithPrivs(dbName, privs string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_user" "test_global" {
  user     = "jdoe-%s"
  host     = "%%"
}

resource "mysql_grant" "test_global" {
  user       = "${mysql_user.test_global.user}"
  host       = "${mysql_user.test_global.host}"
  table      = "*"
  database   = "*"
  privileges = ["SHOW DATABASES"]
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  table      = "tbl"
  database   = "${mysql_database.test.name}"
  privileges = [%s]
}
`, dbName, dbName, dbName, privs)
}

func testAccGrantConfigWithDynamicMySQL8(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  table      = "*"
  database   = "*"
  privileges = ["SHOW DATABASES", "CONNECTION_ADMIN", "SELECT"]
}

`, dbName, dbName)
}

func testAccGrantConfigBasic(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
}
`, dbName, dbName)
}

func testAccGrantConfigBasicWithGrant(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
  grant      = "true"
}
`, dbName, dbName)
}

func testAccGrantConfigExtraHost(dbName string, extraHost bool) string {
	extra := ""
	if extraHost {
		extra = fmt.Sprintf(`
resource "mysql_grant" "test_bet" {
  user       = "${mysql_user.test_bet.user}"
  host       = "${mysql_user.test_bet.host}"
  database   = "mysql"
  privileges = ["DELETE"]
}
		`)
	}

	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test_all" {
  user     = "jdoe-%s"
  host     = "%%"
}

resource "mysql_user" "test" {
  user       = "jdoe-%s"
  host       = "10.1.2.3"
}

resource "mysql_user" "test_bet" {
  user       = "jdoe-%s"
  host       = "10.1.%%.%%"
}

resource "mysql_grant" "test_all" {
  user       = "${mysql_user.test_all.user}"
  host       = "${mysql_user.test_all.host}"
  database   = "mysql"
  privileges = ["UPDATE", "SELECT"]
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "mysql"
  privileges = ["SELECT", "INSERT"]
}
%s
`, dbName, dbName, dbName, dbName, extra)
}

func testAccGrantConfigBroken(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
}

resource "mysql_grant" "test2" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
}
`, dbName, dbName)
}
func testAccGrantConfigRole(dbName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  role       = "${mysql_role.test.name}"
  database   = "${mysql_database.test.name}"
  privileges = ["SELECT", "UPDATE"]
}
`, dbName, roleName)
}

func testAccGrantConfigRoleWithGrantOption(dbName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  role       = "${mysql_role.test.name}"
  database   = "${mysql_database.test.name}"
  privileges = ["SELECT", "UPDATE"]
  grant      = "true"
}
`, dbName, roleName)
}

func testAccGrantConfigRoleToUser(dbName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "jdoe" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  user     = "${mysql_user.jdoe.user}"
  host     = "${mysql_user.jdoe.host}"
  database = "${mysql_database.test.name}"
  roles    = ["${mysql_role.test.name}"]
}
`, dbName, dbName, roleName)
}

func testAccGrantConfigComplexRoleGrants(user string) string {
	return fmt.Sprintf(`
	locals {
		user = "%v"
		host = "%%"
	}

	resource "mysql_user" "user" {
		user = local.user
		host = local.host
	}

	resource "mysql_role" "role1" {
		name = "role1"
	}

	resource "mysql_role" "role2" {
		name = "role2"
	}

	resource "mysql_grant" "adminuser_roles" {
		user     = mysql_user.user.user
		host     = mysql_user.user.host
		database = "*"
		grant    = true
		roles    = [mysql_role.role1.name, mysql_role.role2.name]
	}

	resource "mysql_grant" "role_perms" {
		role       = mysql_role.role1.name
		database   = "mysql"
		privileges = ["SELECT"]
	}

	resource "mysql_grant" "adminuser_privs" {
		user     = mysql_user.user.user
		host     = mysql_user.user.host
		database   = "*"
		grant      = true
		privileges = ["SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "RELOAD", "PROCESS", "REFERENCES", "INDEX", "ALTER", "SHOW DATABASES", "CREATE TEMPORARY TABLES", "LOCK TABLES", "EXECUTE", "REPLICATION SLAVE", "REPLICATION CLIENT", "CREATE VIEW", "SHOW VIEW", "CREATE ROUTINE", "ALTER ROUTINE", "CREATE USER", "EVENT", "TRIGGER"]
	}`, user)
}
func prepareProcedure(dbname string, procedureName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		// Switch to the specified database
		_, err = db.ExecContext(ctx, fmt.Sprintf("USE `%s`", dbname))
		if err != nil {
			return fmt.Errorf("Error selecting database %s: %s", dbname, err)
		}

		// Check if the procedure exists
		var exists int
		checkExistenceSQL := fmt.Sprintf(`
SELECT COUNT(*)
FROM information_schema.ROUTINES
WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'
`)
		err = db.QueryRowContext(ctx, checkExistenceSQL, dbname, procedureName).Scan(&exists)
		if err != nil {
			return fmt.Errorf("Error checking existence of procedure %s: %s", procedureName, err)
		}

		if exists > 0 {
			return nil
		}

		// Create the procedure
		createProcedureSQL := fmt.Sprintf(`
			CREATE PROCEDURE %s()
			BEGIN
				SELECT 1;
			END
			`, procedureName)
		if _, err := db.Exec(createProcedureSQL); err != nil {
			return fmt.Errorf("error reading grant: %s", err)
		}
		return nil
	}
}

func TestAccGrantOnProcedure(t *testing.T) {
	procedureName := "test_procedure"
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-%s", dbName)
	hostName := "%"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName),
				),
			},
			{
				// Create a procedure
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareProcedure(dbName, procedureName),
				),
			},
			{
				Config: testAccGrantConfigProcedure(procedureName, dbName, hostName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProcedureGrant("mysql_grant.test_procedure", userName, hostName, procedureName, true),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "database", fmt.Sprintf("PROCEDURE %s.%s", dbName, procedureName)),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "table", ""), // Ensure table attribute is empty
				),
			},
		},
	})
}

func testAccGrantConfigProcedure(procedureName string, dbName string, hostName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_user" "test_global" {
  user     = "jdoe-%s"
  host     = "%%"
}

resource "mysql_grant" "test_procedure" {
    user       = "jdoe-%s"
    host       = "%s"
    privileges = ["EXECUTE"]
    database   = "PROCEDURE %s.%s"
}
`, dbName, dbName, dbName, dbName, hostName, dbName, procedureName)
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
