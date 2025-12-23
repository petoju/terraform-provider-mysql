package mysql

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"regexp"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccGrant(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-%s", dbName)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
				),
			},
			{
				Config:            testAccGrantConfigBasic(dbName),
				ResourceName:      "mysql_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v@%v@%v", userName, "example.com", dbName, "*"),
			},
		},
	})
}

func TestAccRevokePrivRefresh(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "UPDATE", true, false),
				),
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					revokeUserPrivs(dbName, "UPDATE"),
				),
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "UPDATE", false, false),
				),
			},
			{
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				Config:             testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "UPDATE", false, false),
				),
			},
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "UPDATE", true, false),
				),
			},
		},
	})
}

func TestAccBroken(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigBasic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
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
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
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
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigExtraHost(dbName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test_all", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "host", "%"),
					resource.TestCheckResourceAttr("mysql_grant.test_all", "table", "*"),
				),
			},
			{
				Config: testAccGrantConfigExtraHost(dbName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
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

func TestGrantsOnColumnsWithUserDefaultRole(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userNameA := "jda"
	userNameB := "jdb"
	roleA := "ra"
	roleB := "rb"
	tableName := "tbl"
	passwordA := fmt.Sprintf("password_12345678_ASDF-A-%d", rand.Intn(100))
	passwordB := fmt.Sprintf("password_12345678_ASDF-B-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipMariaDB(t)                     // user-default roles are not supported on MariaDB when using the provider
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0") // user-default roles are not supported on MariaDB when using the provider
			testAccPreCheckSkipTiDB(t)                        // role grants are not supported by this version of MySQL
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigDbOnly(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, tableName),
					insertIntoTable(dbName, tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPassword(dbName, userNameA, passwordA, roleA, tableName, `"SELECT (c1, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					testSqlQueryAsUser(fmt.Sprintf("SELECT `c1`,`c2` FROM `%s`.`%s` WHERE `c1` = 1;", dbName, tableName), userNameA, passwordA),
				),
			},
			{
				Config: testAccGrantConfigWithPassword(dbName, userNameB, passwordB, roleB, tableName, "\"SELECT (`id`,`key`,`order`,`first`,`last`)\""),
				Check: resource.ComposeTestCheckFunc(
					testSqlQueryAsUser(fmt.Sprintf("SELECT `id`,`key`,`order`,`first`,`last` FROM `%s`.`%s` WHERE `id` = 1;", dbName, tableName), userNameB, passwordB),
				),
			},
			{
				Config: testAccGrantConfigDbOnly(dbName), // cleanup
			},
		},
	})
}

func TestAccGrantComplex(t *testing.T) {
	hostName := "example.com"
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-%s", dbName)
	tableName := "tbl"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipTiDB(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT (c1, c2)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, "\"SELECT (`id`,`key`,`order`,`first`,`last`)\"", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1)", "INSERT(c3, c4)", "REFERENCES(c5)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "INSERT (c3,c4)", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT (c1)", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", false, false),
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1)", "INSERT(c4, c3, c2)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", false, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL PRIVILEGES"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1, c2)", "INSERT(c5)", "REFERENCES(c1)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", false, false),
					testAccPrivilege("mysql_grant.test", "DROP", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT(c1,c2)", true, false),
					testAccPrivilege("mysql_grant.test", "INSERT(c5)", true, false),
					testAccPrivilege("mysql_grant.test", "REFERENCES(c1)", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			// Grant SELECT and UPDATE privileges WITH grant option
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT (c1, c2)","UPDATE(c1, c2)"`, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true, true),
					testAccPrivilege("mysql_grant.test", "UPDATE (c1,c2)", true, true),
					testAccPrivilege("mysql_grant.test", "ALL", false, true),
					testAccPrivilege("mysql_grant.test", "DROP", false, true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			// Grant ALL privileges WITH grant option
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL"`, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true, true),
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", false, true),
					testAccPrivilege("mysql_grant.test", "UPDATE (c1,c2)", false, true),
					testAccPrivilege("mysql_grant.test", "DROP", false, true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", hostName),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", tableName),
				),
			},
			// Test import with grant option
			{
				Config:            testAccGrantConfigBasic(dbName),
				ResourceName:      "mysql_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
				// TF (incorrectly) compares items directly without any kind of suppress function.
				// So ALL should be "ALL PRIVILEGES". To avoid the issues, we'll ignore that here.
				ImportStateVerifyIgnore: []string{"privileges.0"},
				ImportStateId:           fmt.Sprintf("%v@%v@%v@%v@", fmt.Sprintf("jdoe-%s", dbName), hostName, dbName, tableName),
			},
			// Finally, revoke all privileges
			{
				Config: testAccGrantConfigNoGrant(dbName),
			},
		},
	})
}

func TestAccGrant_partialRevokeWithGrantOption(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	tableName := "tbl"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipTiDB(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			// Create table first
			{
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, tableName),
				),
			},
			// Grant SELECT, UPDATE, INSERT and GRANT OPTION privileges
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT","UPDATE","INSERT"`, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, true),
					testAccPrivilege("mysql_grant.test", "UPDATE", true, true),
					testAccPrivilege("mysql_grant.test", "INSERT", true, true),
					resource.TestCheckResourceAttr("mysql_grant.test", "grant", "true"),
				),
			},
			// Revoke just the INSERT privilege, it must keep SELECT, UPDATE and GRANT OPTION
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT","UPDATE"`, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, true),
					testAccPrivilege("mysql_grant.test", "UPDATE", true, true),
					testAccPrivilege("mysql_grant.test", "INSERT", false, true),
					resource.TestCheckResourceAttr("mysql_grant.test", "grant", "true"),
				),
			},
			// Revoke UPDATE and GRANT OPTION, it must keep SELECT without GRANT OPTION
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
					testAccPrivilege("mysql_grant.test", "UPDATE", false, false),
					testAccPrivilege("mysql_grant.test", "INSERT", false, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "grant", "false"),
				),
			},
			// Finally, revoke all privileges
			{
				Config: testAccGrantConfigNoGrant(dbName),
			},
		},
	})
}

func TestAccGrantComplexMySQL8(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithDynamicMySQL8(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SHOW DATABASES", true, false),
					testAccPrivilege("mysql_grant.test", "CONNECTION_ADMIN", true, false),
					testAccPrivilege("mysql_grant.test", "FIREWALL_EXEMPT", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
					testAccPrivilege("mysql_grant.test", "ALL PRIVILEGES", false, false),
				),
			},
		},
	})
}

func TestAccGrant_role(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole-exp%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
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
					resource.TestCheckResourceAttr("mysql_grant.test", "grant", "true"),
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
	roleName := fmt.Sprintf("TFRole-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
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

func TestAccGrant_withoutDatabase(t *testing.T) {
	userName := fmt.Sprintf("jdoe-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigWithoutDatabase(userName, roleName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
				),
			},
			{
				Config:            testAccGrantConfigWithoutDatabase(userName, roleName),
				ResourceName:      "mysql_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v@%v@%v", userName, "example.com", "*", "*"),
			},
		},
	})
}

func testAccGrantConfigWithoutDatabase(userName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_user" "test" {
  user     = "%s"
  host     = "example.com"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  user       = mysql_user.test.user
  host       = mysql_user.test.host
  privileges = ["SELECT", "UPDATE"]
}
`, userName, roleName)
}

func TestAccGrant_complexRoleGrants(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigComplexRoleGrants(dbName),
			},
		},
	})
}

func TestAccGrant_roleToRoleGrant(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName1 := fmt.Sprintf("TFRole1-%d", rand.Intn(100))
	roleName2 := fmt.Sprintf("TFRole2-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigRoleToRole(dbName, roleName1, roleName2),
			},
		},
	})
}

func testSqlQueryAsUser(sql, userName, password string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		config := testAccProvider.Meta().(*MySQLConfiguration)
		localConfig := *config
		// Create a deep copy of the Config field to avoid modifying the original
		configCopy := *config.Config
		localConfig.Config = &configCopy
		localConfig.Config.User = userName
		localConfig.Config.Passwd = password

		db, err := connectToMySQL(ctx, &localConfig)
		if err != nil {
			return err
		}
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("error executing SQL: %s", err)
		}

		return nil
	}
}

func prepareTable(dbname string, tableName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE `%s`.`%s`(c1 INT, c2 INT, c3 INT, c4 INT, c5 INT, `id` INT, `key` INT, `order` INT, `type` INT, `status` INT, `created` INT, `updated` INT, `user` INT, `name` INT, `group` INT, `value` INT, `index` INT, `source` INT, `date` INT, `state` INT, `last` INT, `first` INT, `email` INT, `phone` INT, `amount` INT, `reference` INT, `reason` INT, `hash` INT, `team` INT, `case` INT, `uid` INT, `path` INT);", dbname, tableName)); err != nil {
			return fmt.Errorf("error creating table: %s", err)
		}
		return nil
	}
}

func insertIntoTable(dbname string, tableName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf("INSERT INTO `%s`.`%s`(c1, c2, c3, c4, c5, `id`, `key`, `order`, `type`, `status`, `created`, `updated`, `user`, `name`, `group`, `value`, `index`, `source`, `date`, `state`, `last`, `first`, `email`, `phone`, `amount`, `reference`, `reason`, `hash`, `team`, `case`, `uid`, `path`) VALUES (1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1);", dbname, tableName)); err != nil {
			return fmt.Errorf("error inserting into table: %s", err)
		}
		return nil
	}
}

// Test privilege - one can condition it exists or that it doesn't exist.
func testAccPrivilege(rn string, privilege string, expectExists bool, expectGrant bool) resource.TestCheckFunc {
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

		var userOrRole UserOrRole
		if strings.Contains(id[0], "@") {
			parts := strings.Split(id[0], "@")
			userOrRole = UserOrRole{
				Name: parts[0],
				Host: parts[1],
			}
		} else {
			userOrRole = UserOrRole{
				Name: id[0],
			}
		}

		grants, err := showUserGrants(context.Background(), db, userOrRole)
		if err != nil {
			return err
		}

		privilegeNorm := normalizePerms([]string{privilege})[0]

		var expectedGrant MySQLGrant

	Outer:
		for _, grant := range grants {
			grantWithPrivs, ok := grant.(MySQLGrantWithPrivileges)
			if !ok {
				continue
			}
			for _, priv := range grantWithPrivs.GetPrivileges() {
				log.Printf("[DEBUG] Checking grant %s against %s", priv, privilegeNorm)
				if priv == privilegeNorm {
					expectedGrant = grant
					break Outer
				}
			}
		}

		if expectExists != (expectedGrant != nil) {
			if expectedGrant != nil {
				return fmt.Errorf("grant %s found but it was not requested for %s", privilege, userOrRole)
			} else {
				return fmt.Errorf("grant %s not found for %s", privilegeNorm, userOrRole)
			}
		}

		if expectedGrant != nil && expectedGrant.GrantOption() != expectGrant {
			return fmt.Errorf("grant %s found but had incorrect grant option", privilege)
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

func testAccGrantConfigDbOnly(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}
`, dbName)
}

func testAccGrantConfigWithPassword(dbName, userName, password, role, table, privs string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "%s"
  password = "%s"
  host     = "%%"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "grant_privilege_to_role" {
  database   = mysql_database.test.name
  role       = mysql_role.test.name
  table      = "%s"
  privileges = [%s]
}

resource "mysql_grant" "grant_role_to_user" {
  database   = mysql_database.test.name
  user       = mysql_user.test.user
  host       = "%%"
  roles      = [mysql_role.test.name]
}

resource "mysql_default_roles" "test" {
  user  = mysql_user.test.user
  host  = "%%"
  roles = mysql_grant.grant_role_to_user.roles
}

`, dbName, userName, password, role, table, privs)
}

func testAccGrantConfigWithPrivs(dbName, privs string, grantOption bool) string {

	grantOptionStr := "false"
	if grantOption {
		grantOptionStr = "true"
	}

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
  grant      = %s
}
`, dbName, dbName, dbName, privs, grantOptionStr)
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
  privileges = ["SHOW DATABASES", "CONNECTION_ADMIN", "SELECT", "FIREWALL_EXEMPT"]
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

func testAccGrantConfigRoleToRole(dbName string, roleName1 string, roleName2 string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_role" "test1" {
  name = "%s"
}

resource "mysql_role" "test2" {
  name = "%s"
}

resource "mysql_grant" "test" {
  role       = "${mysql_role.test1.name}"
  database   = "${mysql_database.test.name}"
  roles      = ["${mysql_role.test2.name}"]
}
`, dbName, roleName1, roleName2)
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
		log.Printf("[DEBUG] SQL: %s", dbname)
		if err != nil {
			return fmt.Errorf("error selecting database %s: %s", dbname, err)
		}

		// Check if the procedure exists
		var exists int
		checkExistenceSQL := fmt.Sprintf(`
SELECT COUNT(*)
FROM information_schema.ROUTINES
WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'
`)
		log.Printf("[DEBUG] SQL: %s", checkExistenceSQL)
		err = db.QueryRowContext(ctx, checkExistenceSQL, dbname, procedureName).Scan(&exists)
		if err != nil {
			return fmt.Errorf("error checking existence of procedure %s: %s", procedureName, err)
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
		log.Printf("[DEBUG] SQL: %s", createProcedureSQL)
		if _, err := db.Exec(createProcedureSQL); err != nil {
			return fmt.Errorf("error creating procedure: %s", err)
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
		PreCheck:          func() { testAccPreCheckSkipTiDB(t); testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, "tbl"),
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
				Config: testAccGrantConfigProcedureWithTable(procedureName, dbName, hostName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProcedureGrant("mysql_grant.test_procedure", userName, hostName, procedureName, true),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "host", hostName),
					// Note: The database and table name do not change. This is to preserve legacy functionality.
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "database", fmt.Sprintf("PROCEDURE %s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "table", procedureName),
				),
			},
			{
				// Remove the grant
				Config: testAccGrantConfigNoGrant(dbName),
			},
			{
				Config: testAccGrantConfigProcedureWithDatabase(procedureName, dbName, hostName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckProcedureGrant("mysql_grant.test_procedure", userName, hostName, procedureName, true),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "user", userName),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "host", hostName),
					// Note: The database and table name do not change. This is to preserve legacy functionality.
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "database", fmt.Sprintf("PROCEDURE %s.%s", dbName, procedureName)),
					resource.TestCheckResourceAttr("mysql_grant.test_procedure", "table", "*"),
				),
			},
		},
	})
}

func testAccGrantConfigProcedureWithTable(procedureName string, dbName string, hostName string) string {
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
    database   = "PROCEDURE %s"
    table      = "%s"
}
`, dbName, dbName, dbName, dbName, hostName, dbName, procedureName)
}

func testAccGrantConfigProcedureWithDatabase(procedureName string, dbName string, hostName string) string {
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

		// Query to show grants for the specific user
		query := fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", userName, hostName)
		log.Printf("[DEBUG] SQL: %s", query)

		// Use db.Query to execute the query
		rows, err := db.Query(query)
		if err != nil {
			return err
		}
		defer rows.Close()

		// Variable to track if the required privilege is found
		found := false

		// Iterate through the results
		for rows.Next() {
			var grant string
			if err := rows.Scan(&grant); err != nil {
				return err
			}

			// Check if the grant string contains the necessary privilege
			// Adjust the following line according to the exact format of your GRANT statement
			if strings.Contains(grant, fmt.Sprintf("`%s`", procedureName)) && strings.Contains(grant, "EXECUTE") {
				found = true
				break
			}
		}

		// Check if there was an error during iteration
		if err := rows.Err(); err != nil {
			return err
		}

		// Compare the result with the expected outcome
		if found != expected {
			return fmt.Errorf("grant for procedure %s does not match expected state: %v", procedureName, expected)
		}

		return nil
	}
}

func revokeUserPrivs(dbname string, privs string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		// Revoke privileges for this user
		revokeAllSql := fmt.Sprintf("REVOKE %s ON `%s`.* FROM `jdoe-%s`@`example.com`;", privs, dbname, dbname)
		log.Printf("[DEBUG] SQL: %s", revokeAllSql)
		if _, err := db.Exec(revokeAllSql); err != nil {
			return fmt.Errorf("error revoking grant: %s", err)
		}
		return nil
	}
}

func TestAllowDuplicateUsersDifferentTables(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))

	duplicateUserConfig := fmt.Sprintf(`
	resource "mysql_database" "test" {
	  name = "%s"
	}

	resource "mysql_user" "test" {
	  user     = "jdoe-%s"
	  host     = "example.com"
	}

	resource "mysql_grant" "grant1" {
	  user       = "${mysql_user.test.user}"
	  host       = "${mysql_user.test.host}"
	  database   = "${mysql_database.test.name}"
          table      = "table1"
	  privileges = ["UPDATE", "SELECT"]
	}

	resource "mysql_grant" "grant2" {
	  user       = "${mysql_user.test.user}"
	  host       = "${mysql_user.test.host}"
	  database   = "${mysql_database.test.name}"
	  table      = "table2"
	  privileges = ["UPDATE", "SELECT"]
	}
	`, dbName, dbName)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, "table1"),
					prepareTable(dbName, "table2"),
				),
			},
			{
				Config: duplicateUserConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant1", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.grant1", "table", "table1"),
					testAccPrivilege("mysql_grant.grant2", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.grant2", "table", "table2"),
				),
			},
			{
				RefreshState: true,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant1", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.grant1", "table", "table1"),
					testAccPrivilege("mysql_grant.grant2", "SELECT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.grant2", "table", "table2"),
				),
			},
		},
	})
}

func TestDisallowDuplicateUsersSameTable(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))

	duplicateUserConfig := fmt.Sprintf(`
	resource "mysql_database" "test" {
	  name = "%s"
	}

	resource "mysql_user" "test" {
	  user     = "jdoe-%s"
	  host     = "example.com"
	}

	resource "mysql_grant" "grant1" {
	  user       = "${mysql_user.test.user}"
	  host       = "${mysql_user.test.host}"
	  database   = "${mysql_database.test.name}"
          table      = "table1"
	  privileges = ["UPDATE", "SELECT"]
	}

	resource "mysql_grant" "grant2" {
	  user       = "${mysql_user.test.user}"
	  host       = "${mysql_user.test.host}"
	  database   = "${mysql_database.test.name}"
	  table      = "table1"
	  privileges = ["UPDATE", "SELECT"]
	}
	`, dbName, dbName)

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigNoGrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName, "table1"),
				),
			},
			{
				Config:      duplicateUserConfig,
				ExpectError: regexp.MustCompile("already has"),
			},
		},
	})
}

// TestModifyPrivileges explicitly verifies the correct and incorrect ways of modifying privileges.
// It tests adding privileges by augmenting the existing grant (correct way).
// It also tests that dynamic privileges configured on the global (`*`) database can coexist with grants on specific databases.
func TestModifyPrivileges(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-modify-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole-modify-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-modify-%s", dbName)

	onePrivilegeConfig := getGrantsSampleWithPrivileges(roleName, dbName, userName, `"SELECT"`)
	twoPrivilegesConfig := getGrantsSampleWithPrivileges(roleName, dbName, userName, `"SELECT", "UPDATE"`)
	additionalStaticPrivilegeConfig := twoPrivilegesConfig + getAdditionalGrantSample(dbName, `"INSERT"`)
	threePrivilegesConfig := getGrantsSampleWithPrivileges(roleName, dbName, userName, `"SELECT", "UPDATE", "INSERT"`)
	// Configuring dynamic privilege on global (`*`) database alongside specific database grants
	additionalDynamicPrivilegeConfigFlushTables := threePrivilegesConfig + getAdditionalGrantSample("*", `"FLUSH_TABLES"`)
	additionalDynamicPrivilegeConfigShowRoutine := threePrivilegesConfig + getAdditionalGrantSample("*", `"SHOW_ROUTINE"`)
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigNoGrant(dbName),
			},
			{
				Config: onePrivilegeConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant", "SELECT", true, false),
					testAccPrivilege("mysql_grant.grant", "UPDATE", false, false),
					testAccPrivilege("mysql_grant.grant", "INSERT", false, false),
				),
			},
			{
				// Correct way: augment existing grant with additional privileges
				Config: twoPrivilegesConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant", "SELECT", true, false),
					testAccPrivilege("mysql_grant.grant", "UPDATE", true, false),
					testAccPrivilege("mysql_grant.grant", "INSERT", false, false),
				),
			},
			{
				// Incorrect way: create a new conflicting grant (expected to fail)
				Config:      additionalStaticPrivilegeConfig,
				ExpectError: regexp.MustCompile("already has"),
			},
			{
				// Correct way: augment existing grant with additional privileges
				Config: threePrivilegesConfig,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant", "SELECT", true, false),
					testAccPrivilege("mysql_grant.grant", "UPDATE", true, false),
					testAccPrivilege("mysql_grant.grant", "INSERT", true, false),
				),
			},

			// Testing coexistence of dynamic privilege on global (`*`) database with specific database grants

			{
				Config: additionalDynamicPrivilegeConfigFlushTables,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant", "SELECT", true, false),
					testAccPrivilege("mysql_grant.grant", "UPDATE", true, false),
					testAccPrivilege("mysql_grant.grant", "INSERT", true, false),
					testAccPrivilege("mysql_grant.additional_grant", "FLUSH_TABLES", true, false),
					testAccPrivilege("mysql_grant.additional_grant", "SHOW_ROUTINE", false, false),
				),
			},
			{
				Config: additionalDynamicPrivilegeConfigShowRoutine,
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.grant", "SELECT", true, false),
					testAccPrivilege("mysql_grant.grant", "UPDATE", true, false),
					testAccPrivilege("mysql_grant.grant", "INSERT", true, false),
					testAccPrivilege("mysql_grant.additional_grant", "FLUSH_TABLES", false, false),
					testAccPrivilege("mysql_grant.additional_grant", "SHOW_ROUTINE", true, false),
				),
			},
		},
	})
}

func getGrantsSampleWithPrivileges(roleName string, dbName string, userName string, privileges string) string {
	return fmt.Sprintf(`

	resource "mysql_role" "role" {
	  name     = "%s"
	}

	resource "mysql_grant" "grant" {
	  role       = "${mysql_role.role.name}"
	  database   = "%s"
	  privileges = [%s]
	}

	resource "mysql_user" "user" {
	  user = "%s"
	  host = "%%"
	}

	resource "mysql_grant" "user_grant" {
	  user       = "${mysql_user.user.user}"
	  host       = "${mysql_user.user.host}"
	  database   = "%s"
	  roles	     = ["${mysql_role.role.name}"]
	}

	`, roleName, dbName, privileges, userName, dbName)
}

func getAdditionalGrantSample(dbName string, privileges string) string {
	return fmt.Sprintf(`

    resource "mysql_grant" "additional_grant" {
      role       = "${mysql_role.role.name}"
      database   = "%s"
      privileges = [%s]
    }
    `, dbName, privileges)
}
