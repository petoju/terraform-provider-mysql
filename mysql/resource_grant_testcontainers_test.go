//go:build testcontainers
// +build testcontainers

package mysql

import (
	"fmt"
	"math/rand"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccGrant_WithTestcontainers tests basic grant functionality
// Uses shared container set up in TestMain
func TestAccGrant_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-%s", dbName)
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestAccRevokePrivRefresh_WithTestcontainers tests privilege revocation and refresh
func TestAccRevokePrivRefresh_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestAccBroken_WithTestcontainers tests error handling for duplicate grants
func TestAccBroken_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

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

// TestAccDifferentHosts_WithTestcontainers tests grants with different hosts
func TestAccDifferentHosts_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestAccGrantComplex_WithTestcontainers tests complex grant scenarios
func TestAccGrantComplex_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT (c1, c2)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"DROP", "SELECT (c1)", "INSERT(c3, c4)", "REFERENCES(c5)"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "INSERT (c3,c4)", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT (c1)", true, false),
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", false, false),
					testAccPrivilege("mysql_grant.test", "REFERENCES (c5)", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"ALL PRIVILEGES"`, false),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "ALL", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigWithPrivs(dbName, `"SELECT (c1, c2)","UPDATE(c1, c2)"`, true),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "SELECT (c1,c2)", true, true),
					testAccPrivilege("mysql_grant.test", "UPDATE (c1,c2)", true, true),
					testAccPrivilege("mysql_grant.test", "ALL", false, true),
					testAccPrivilege("mysql_grant.test", "DROP", false, true),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
				),
			},
			{
				Config: testAccGrantConfigNoGrant(dbName),
			},
		},
	})
}

// TestAccGrantComplexMySQL8_WithTestcontainers tests MySQL 8.0 specific grants
func TestAccGrantComplexMySQL8_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigWithDynamicMySQL8(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilege("mysql_grant.test", "CONNECTION_ADMIN", true, false),
					testAccPrivilege("mysql_grant.test", "FIREWALL_EXEMPT", true, false),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", "*"),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
				),
			},
		},
	})
}

// TestAccGrant_role_WithTestcontainers tests role grants (requires MySQL 8.0+)
func TestAccGrant_role_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole-exp%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestAccGrant_roleToUser_WithTestcontainers tests granting roles to users (requires MySQL 8.0+)
func TestAccGrant_roleToUser_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestAccGrant_complexRoleGrants_WithTestcontainers tests complex role grant scenarios (requires MySQL 8.0+)
func TestAccGrant_complexRoleGrants_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfigComplexRoleGrants(dbName),
			},
		},
	})
}

// TestAccGrantOnProcedure_WithTestcontainers tests procedure grants
func TestAccGrantOnProcedure_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	procedureName := "test_procedure"
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	userName := fmt.Sprintf("jdoe-%s", dbName)
	hostName := "%"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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
				),
			},
		},
	})
}

// TestAllowDuplicateUsersDifferentTables_WithTestcontainers tests allowing duplicate grants on different tables
func TestAllowDuplicateUsersDifferentTables_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

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
		PreCheck:          func() { testAccPreCheck(t) },
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

// TestDisallowDuplicateUsersSameTable_WithTestcontainers tests disallowing duplicate grants on same table
func TestDisallowDuplicateUsersSameTable_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

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
		PreCheck:          func() { testAccPreCheck(t) },
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
