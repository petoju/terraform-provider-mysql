package mysql

import (
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccGrant(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_basic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfig_basic(dbName),
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
				Config: testAccGrantConfig_basic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
			{
				Config: testAccGrantConfig_basicWithGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
				),
			},
			{
				Config: testAccGrantConfig_basic(dbName),
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
				Config: testAccGrantConfig_basic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
			{
				Config:      testAccGrantConfig_broken(dbName),
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
				Config: testAccGrantConfig_extraHost(dbName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test_all", "SELECT", true),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "host", "%"),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfig_extraHost(dbName, true),
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
		PreCheck:     func() { testAccPreCheckSkipTiDB(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfig_nogrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"SELECT (c1, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1)", "INSERT(c3, c4)", "REFERENCES(c5)"`),
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
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1)", "INSERT(c4, c3, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"ALL PRIVILEGES"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"ALL"`),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1, c2)", "INSERT(c5)", "REFERENCES(c1)"`),
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

func TestAccGrant_role(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
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
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_role(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
			{
				Config: testAccGrantConfig_roleWithGrantOption(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
			{
				Config: testAccGrantConfig_role(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
		},
	})
}

func TestAccGrant_roleToUser(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
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
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_roleToUser(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "roles.#", "1"),
				),
			},
		},
	})
}

func prepareTable(dbname string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
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

		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
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

		grants, err := showUserGrants(db, userOrRole)
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
	db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
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
		defer rows.Close()

		if rows.Next() {
			return fmt.Errorf("grant still exists for: %s", userOrRole)
		}
	}
	return nil
}

func testAccGrantConfig_nogrant(dbName string) string {
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

func testAccGrantConfig_with_privs(dbName, privs string) string {
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

func testAccGrantConfig_basic(dbName string) string {
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

func testAccGrantConfig_basicWithGrant(dbName string) string {
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

func testAccGrantConfig_extraHost(dbName string, extraHost bool) string {
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

func testAccGrantConfig_broken(dbName string) string {
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
func testAccGrantConfig_role(dbName string, roleName string) string {
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

func testAccGrantConfig_roleWithGrantOption(dbName string, roleName string) string {
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

func testAccGrantConfig_roleToUser(dbName string, roleName string) string {
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
