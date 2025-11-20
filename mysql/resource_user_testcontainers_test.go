//go:build testcontainers
// +build testcontainers

package mysql

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccUser_basic_WithTestcontainers tests the mysql_user resource
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain
func TestAccUser_basic_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "plaintext_password", hashSum("password")),
					resource.TestCheckResourceAttr("mysql_user.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccUserConfig_ssl,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "plaintext_password", hashSum("password")),
					resource.TestCheckResourceAttr("mysql_user.test", "tls_option", "SSL"),
				),
			},
			{
				Config: testAccUserConfig_newPass,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "plaintext_password", hashSum("password2")),
					resource.TestCheckResourceAttr("mysql_user.test", "tls_option", "NONE"),
				),
			},
		},
	})
}

// TestAccUser_auth_WithTestcontainers tests auth plugin functionality
// Requires MySQL (not TiDB/MariaDB/RDS) with mysql_no_login plugin
// Uses shared container set up in TestMain
// Note: mysql_no_login plugin may not be available in all MySQL distributions
func TestAccUser_auth_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// Check if mysql_no_login plugin is available
			ctx := context.Background()
			db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
			if err != nil {
				t.Fatalf("Cannot connect to DB: %v", err)
			}
			// Don't close - connection is cached and shared

			// Check if plugin exists
			var pluginName string
			err = db.QueryRowContext(ctx, "SELECT PLUGIN_NAME FROM INFORMATION_SCHEMA.PLUGINS WHERE PLUGIN_NAME = 'mysql_no_login'").Scan(&pluginName)
			if err != nil {
				t.Skip("mysql_no_login plugin is not available in this MySQL distribution")
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_auth_iam_plugin,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "mysql_no_login"),
				),
			},
			{
				Config: testAccUserConfig_auth_native,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "mysql_native_password"),
				),
			},
			{
				Config: testAccUserConfig_auth_iam_plugin,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "mysql_no_login"),
				),
			},
		},
	})
}

// TestAccUser_authConnect_WithTestcontainers tests password authentication
// Requires MySQL (not TiDB/MariaDB/RDS)
// Uses shared container set up in TestMain
func TestAccUser_authConnect_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_newPass,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "random"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
			{
				Config: testAccUserConfig_newPass,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
			{
				Config: testAccUserConfig_newPass,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
		},
	})
}

// TestAccUser_authConnectRetainOldPassword_WithTestcontainers tests retain_old_password
// Requires MySQL 8.0.14+
// Uses shared container set up in TestMain
func TestAccUser_authConnectRetainOldPassword_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_newPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_newNewPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
		},
	})
}

// TestAccUser_deprecated_WithTestcontainers tests deprecated password attribute
// Uses shared container set up in TestMain
func TestAccUser_deprecated_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_deprecated,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "password", "password"),
				),
			},
			{
				Config: testAccUserConfig_deprecated_newPass,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "password", "password2"),
				),
			},
		},
	})
}
