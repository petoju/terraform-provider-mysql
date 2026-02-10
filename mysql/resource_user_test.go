package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccUser_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipMariaDB(t) },
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

func TestAccUser_auth(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipTiDB(t); testAccPreCheckSkipMariaDB(t); testAccPreCheckSkipRds(t) },
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
				Config: testAccUserConfig_auth_native_plaintext,
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

func TestAccUser_auth_mysql8(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipTiDB(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.14")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_auth_caching_sha2_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "jdoe"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "caching_sha2_password"),
				),
			},
		},
	})
}

func TestAccUser_auth_string_hash_mysql8(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipTiDB(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.14")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_auth_caching_sha2_password_hex_no_prefix,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "hex"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "caching_sha2_password"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_string_hex", "0x244124303035242931790D223576077A1446190832544A61301A256D5245316662534E56317A434A6A625139555A5642486F4B7A6F675266656B583330744379783134313239"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_hex_no_prefix,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("hex", "password"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_hex_with_prefix,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "hex"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "caching_sha2_password"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_string_hex", "0x244124303035246C4F1E0D5D1631594F5C56701F3D327D073A724C706273307A5965516C7756576B317A5064687A715347765747746B66746A5A4F6E384C41756E6750495330"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_hex_updated,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "hex"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_plugin", "caching_sha2_password"),
					resource.TestCheckResourceAttr("mysql_user.test", "auth_string_hex", "0x244124303035242931790D223576077A1446190832544A61301A256D5245316662534E56317A434A6A625139555A5642486F4B7A6F675266656B583330744379783134313239"),
				),
			},
		},
	})
}

func TestAccUser_auth_mysql8_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipTiDB(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.14")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_auth_caching_sha2_password_hex_invalid,
				ExpectError: regexp.MustCompile(`invalid hex character 'g'`),
			},
			{
				Config:      testAccUserConfig_auth_caching_sha2_password_hex_odd_length,
				ExpectError: regexp.MustCompile(`hex string must have even length`),
			},
			{
				Config:      testAccUserConfig_auth_both_string_fields,
				ExpectError: regexp.MustCompile(`"auth_string_hex": conflicts with auth_string_hashed`),
			},
		},
	})
}

func TestAccUser_authConnect(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipTiDB(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
		},
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

func TestAccUser_authConnectRetainOldPassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.14")
		},
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
			{
				Config: testAccUserConfig_auth_native_plaintext_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_auth_native_plaintext_newPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_auth_native_plaintext_newNewPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_newPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_newNewPass_retain_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
		},
	})
}

func TestAccUser_authConnectDiscardOldPassword(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.14")
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_basic_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_newPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_deleteOldPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
			{
				Config: testAccUserConfig_auth_native_plaintext_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_auth_native_plaintext_newPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_auth_native_plaintext_deleteOldPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_newPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
					testAccUserAuthValid("jdoe", "password2"),
				),
			},
			{
				Config: testAccUserConfig_auth_caching_sha2_password_deleteOldPass_discard_old_password,
				Check: resource.ComposeTestCheckFunc(
					testAccUserAuthValid("jdoe", "password"),
				),
				ExpectError: regexp.MustCompile(`.*Access denied for user 'jdoe'.*`),
			},
		},
	})
}

func TestAccUser_deprecated(t *testing.T) {
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

func TestAccUser_passwordWO(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipMariaDB(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_password_wo,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "wo"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "password_wo_version", "1"),
					testAccUserAuthValid("wo", "secret1"),
				),
			},
			{
				Config: testAccUserConfig_password_wo_update,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "wo"),
					resource.TestCheckResourceAttr("mysql_user.test", "host", "%"),
					resource.TestCheckResourceAttr("mysql_user.test", "password_wo_version", "2"),
					testAccUserAuthValid("wo", "secret2"),
				),
			},
		},
	})
}

func testAccUserExists(rn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("user id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		stmtSQL := fmt.Sprintf("SELECT count(*) from mysql.user where CONCAT(user, '@', host) = '%s'", rs.Primary.ID)
		log.Println("[DEBUG] Executing statement:", stmtSQL)
		var count int
		err = db.QueryRow(stmtSQL).Scan(&count)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("expected 1 row reading user but got no rows")
			}
			return fmt.Errorf("error reading user: %s", err)
		}

		return nil
	}
}

func testAccUserAuthExists(rn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("user id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		stmtSQL := fmt.Sprintf("SELECT count(*) from mysql.user where CONCAT(user, '@', host) = '%s' and plugin = 'mysql_no_login'", rs.Primary.ID)
		log.Println("[DEBUG] Executing statement:", stmtSQL)
		var count int
		err = db.QueryRow(stmtSQL).Scan(&count)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("expected 1 row reading user but got no rows")
			}
			return fmt.Errorf("error reading user: %s", err)
		}

		return nil
	}
}

func testAccUserAuthValid(user string, password string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		userConf := testAccProvider.Meta().(*MySQLConfiguration)
		userConf.Config.User = user
		userConf.Config.Passwd = password

		ctx := context.Background()
		connection, err := createNewConnection(ctx, userConf)
		if err != nil {
			return fmt.Errorf("could not create new connection: %v", err)
		}

		connection.Db.Close()

		return nil
	}
}

func testAccUserCheckDestroy(s *terraform.State) error {
	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mysql_user" {
			continue
		}

		stmtSQL := fmt.Sprintf("SELECT user from mysql.user where CONCAT(user, '@', host) = '%s'", rs.Primary.ID)
		log.Println("[DEBUG] Executing statement:", stmtSQL)
		rows, err := db.Query(stmtSQL)
		if err != nil {
			return fmt.Errorf("error issuing query: %s", err)
		}
		haveNext := rows.Next()
		rows.Close()
		if haveNext {
			return fmt.Errorf("user still exists after destroy")
		}
	}
	return nil
}

const testAccUserConfig_basic = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "%"
    plaintext_password = "password"
}
`

const testAccUserConfig_ssl = `
resource "mysql_user" "test" {
	user = "jdoe"
	host = "example.com"
	plaintext_password = "password"
	tls_option = "SSL"
}
`

const testAccUserConfig_newPass = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "%"
    plaintext_password = "password2"
}
`

const testAccUserConfig_deprecated = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "example.com"
    password = "password"
}
`

const testAccUserConfig_deprecated_newPass = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "example.com"
    password = "password2"
}
`

const testAccUserConfig_auth_iam_plugin = `
resource "mysql_user" "test" {
    user        = "jdoe"
    host        = "example.com"
    auth_plugin = "mysql_no_login"
}
`

const testAccUserConfig_auth_native = `
resource "mysql_user" "test" {
    user        = "jdoe"
    host        = "example.com"
    auth_plugin = "mysql_native_password"

    # Hash of "password"
    auth_string_hashed = "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19"
}
`

const testAccUserConfig_auth_native_plaintext = `
resource "mysql_user" "test" {
    user               = "jdoe"
    host               = "example.com"
    auth_plugin        = "mysql_native_password"
    plaintext_password = "password"
}
`

const testAccUserConfig_auth_caching_sha2_password = `
resource "mysql_user" "test" {
    user               = "jdoe"
    host               = "example.com"
    auth_plugin        = "caching_sha2_password"
    plaintext_password = "password"
}
`

const testAccUserConfig_auth_caching_sha2_password_hex_no_prefix = `
resource "mysql_user" "test" {
    user            = "hex"
    host            = "%"
    auth_plugin     = "caching_sha2_password"
    auth_string_hex = "244124303035242931790D223576077A1446190832544A61301A256D5245316662534E56317A434A6A625139555A5642486F4B7A6F675266656B583330744379783134313239"
}
`
const testAccUserConfig_auth_caching_sha2_password_hex_with_prefix = `
resource "mysql_user" "test" {
    user            = "hex"
    host            = "%"
    auth_plugin     = "caching_sha2_password"
    auth_string_hex = "0x244124303035246C4F1E0D5D1631594F5C56701F3D327D073A724C706273307A5965516C7756576B317A5064687A715347765747746B66746A5A4F6E384C41756E6750495330"
}
`
const testAccUserConfig_auth_caching_sha2_password_hex_updated = `
resource "mysql_user" "test" {
    user            = "hex"
    host            = "%"
    auth_plugin     = "caching_sha2_password"
    auth_string_hex = "244124303035242931790D223576077A1446190832544A61301A256D5245316662534E56317A434A6A625139555A5642486F4B7A6F675266656B583330744379783134313239"
}
`
const testAccUserConfig_auth_caching_sha2_password_hex_invalid = `
resource "mysql_user" "test" {
    user            = "jdoe"
    host            = "example.com"
    auth_plugin     = "caching_sha2_password"
    auth_string_hex = "0x244124303035246g4f1e0d5d1631594f5c56701f3d327d073a724c706273307a5965516c7756"
}
`
const testAccUserConfig_auth_caching_sha2_password_hex_odd_length = `
resource "mysql_user" "test" {
    user            = "jdoe"
    host            = "example.com"
    auth_plugin     = "caching_sha2_password"
    auth_string_hex = "0x244124303035246c4f1e0d5d1631594f5c56701f3d327d073a724c706273307a5965516c775"
}
`
const testAccUserConfig_auth_both_string_fields = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "example.com"
    auth_plugin         = "caching_sha2_password"
    auth_string_hashed  = "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19"
    auth_string_hex     = "0x244124303035246c4f1e0d5d1631594f5c56701f3d327d073a724c706273307a5965516c7756"
}
`

const testAccUserConfig_basic_retain_old_password = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "%"
    plaintext_password = "password"
    retain_old_password = true
}
`

const testAccUserConfig_newPass_retain_old_password = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "%"
    plaintext_password = "password2"
    retain_old_password = true
}
`

const testAccUserConfig_newNewPass_retain_old_password = `
resource "mysql_user" "test" {
    user = "jdoe"
    host = "%"
    plaintext_password = "password3"
    retain_old_password = true
}
`

const testAccUserConfig_auth_native_plaintext_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "mysql_native_password"
    plaintext_password  = "password"
    retain_old_password = true
}
`

const testAccUserConfig_auth_native_plaintext_newPass_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "mysql_native_password"
    plaintext_password  = "password2"
    retain_old_password = true
}
`

const testAccUserConfig_auth_native_plaintext_newNewPass_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "mysql_native_password"
    plaintext_password  = "password3"
    retain_old_password = true
}
`

const testAccUserConfig_auth_caching_sha2_password_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "caching_sha2_password"
    plaintext_password  = "password"
    retain_old_password = true
}
`

const testAccUserConfig_auth_caching_sha2_password_newPass_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "caching_sha2_password"
    plaintext_password  = "password2"
    retain_old_password = true
}
`

const testAccUserConfig_auth_caching_sha2_password_newNewPass_retain_old_password = `
resource "mysql_user" "test" {
    user                = "jdoe"
    host                = "%"
    auth_plugin         = "caching_sha2_password"
    plaintext_password  = "password3"
    retain_old_password = true
}
`

const testAccUserConfig_basic_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    plaintext_password   = "password"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_newPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = false
}
`

const testAccUserConfig_deleteOldPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_auth_native_plaintext_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "mysql_native_password"
    plaintext_password   = "password"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_auth_native_plaintext_newPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "mysql_native_password"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = false
}
`

const testAccUserConfig_auth_native_plaintext_deleteOldPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "mysql_native_password"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_auth_caching_sha2_password_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "caching_sha2_password"
    plaintext_password   = "password"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_auth_caching_sha2_password_newPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "caching_sha2_password"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = false
}
`

const testAccUserConfig_auth_caching_sha2_password_deleteOldPass_discard_old_password = `
resource "mysql_user" "test" {
    user                 = "jdoe"
    host                 = "%"
    auth_plugin          = "caching_sha2_password"
    plaintext_password   = "password2"
    retain_old_password  = true
    discard_old_password = true
}
`

const testAccUserConfig_password_wo = `
resource "mysql_user" "test" {
    user = "wo"
    host = "%"
    password_wo = "secret1"
    password_wo_version = 1
}
`

const testAccUserConfig_password_wo_update = `
resource "mysql_user" "test" {
    user = "wo"
    host = "%"
    password_wo = "secret2"
    password_wo_version = 2
}
`

// Resource limits tests
func TestAccUser_resourceLimits(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipTiDB(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_resourceLimits,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "limited_user"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_user_connections", "10"),
					testAccUserResourceLimitsMaxConn("limited_user", "%", 10),
				),
			},
			{
				Config: testAccUserConfig_resourceLimitsUpdated,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_user_connections", "20"),
					testAccUserResourceLimitsMaxConn("limited_user", "%", 20),
				),
			},
			{
				Config: testAccUserConfig_resourceLimitsRemoved,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					testAccUserResourceLimitsMaxConn("limited_user", "%", 0),
				),
			},
		},
	})
}

// MariaDB-specific test with MAX_STATEMENT_TIME
func TestAccUser_resourceLimitsMariaDB(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckRequireMariaDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_resourceLimitsMariaDB,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "limited_user_mariadb"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_user_connections", "15"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_statement_time", "30.5"),
					testAccUserResourceLimitsMariaDB("limited_user_mariadb", "%", 15, 30.5),
				),
			},
			{
				Config: testAccUserConfig_resourceLimitsMariaDBUpdated,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_user_connections", "25"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_statement_time", "45.5"),
					testAccUserResourceLimitsMariaDB("limited_user_mariadb", "%", 25, 45.5),
				),
			},
			{
				Config: testAccUserConfig_resourceLimitsMariaDBRemoved,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					testAccUserResourceLimitsMariaDB("limited_user_mariadb", "%", 0, 0),
				),
			},
		},
	})
}

// Test fractional MAX_STATEMENT_TIME on MariaDB
func TestAccUser_resourceLimitsFractional(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckRequireMariaDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserConfig_resourceLimitsFractional,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user.test", "user", "fractional_user"),
					resource.TestCheckResourceAttr("mysql_user.test", "max_statement_time", "0.01"),
					testAccUserResourceLimitsMariaDB("fractional_user", "%", 0, 0.01),
				),
			},
		},
	})
}

// Test that MAX_STATEMENT_TIME fails on MySQL
func TestAccUser_resourceLimitsErrorOnMySQL(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipTiDB(t) // TiDB doesn't support resource limits, test is MySQL-specific
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_resourceLimitsErrorMySQL,
				ExpectError: regexp.MustCompile("MAX_STATEMENT_TIME is only supported on MariaDB"),
			},
		},
	})
}

// Test that MAX_USER_CONNECTIONS fails on TiDB with a clear error message
func TestAccUser_resourceLimitsErrorOnTiDB(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t) // This test is TiDB-specific
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccUserConfig_resourceLimitsErrorTiDB,
				ExpectError: regexp.MustCompile("MAX_USER_CONNECTIONS is not supported on TiDB"),
			},
		},
	})
}

// Helper function to verify MAX_USER_CONNECTIONS in database
func testAccUserResourceLimitsMaxConn(user, host string, expectedMaxConn int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		var maxUserConn int

		query := fmt.Sprintf("SELECT max_user_connections FROM mysql.user WHERE user='%s' AND host='%s'", user, host)
		err = db.QueryRow(query).Scan(&maxUserConn)
		if err != nil {
			return fmt.Errorf("error reading user resource limits: %s", err)
		}

		if maxUserConn != expectedMaxConn {
			return fmt.Errorf("expected max_user_connections %d, got %d", expectedMaxConn, maxUserConn)
		}

		return nil
	}
}

// Helper function to verify both MAX_USER_CONNECTIONS and MAX_STATEMENT_TIME in MariaDB
func testAccUserResourceLimitsMariaDB(user, host string, expectedMaxConn int, expectedMaxStmt float64) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		var maxUserConn int
		var maxStmtTime float64

		query := fmt.Sprintf("SELECT max_user_connections, max_statement_time FROM mysql.user WHERE user='%s' AND host='%s'", user, host)
		err = db.QueryRow(query).Scan(&maxUserConn, &maxStmtTime)
		if err != nil {
			return fmt.Errorf("error reading user resource limits: %s", err)
		}

		if maxUserConn != expectedMaxConn {
			return fmt.Errorf("expected max_user_connections %d, got %d", expectedMaxConn, maxUserConn)
		}

		if maxStmtTime != expectedMaxStmt {
			return fmt.Errorf("expected max_statement_time %f, got %f", expectedMaxStmt, maxStmtTime)
		}

		return nil
	}
}

const testAccUserConfig_resourceLimits = `
resource "mysql_user" "test" {
    user                 = "limited_user"
    host                 = "%"
    plaintext_password   = "password"
    max_user_connections = 10
}
`

const testAccUserConfig_resourceLimitsUpdated = `
resource "mysql_user" "test" {
    user                 = "limited_user"
    host                 = "%"
    plaintext_password   = "password"
    max_user_connections = 20
}
`

const testAccUserConfig_resourceLimitsRemoved = `
resource "mysql_user" "test" {
    user               = "limited_user"
    host               = "%"
    plaintext_password = "password"
}
`

const testAccUserConfig_resourceLimitsMariaDB = `
resource "mysql_user" "test" {
    user                 = "limited_user_mariadb"
    host                 = "%"
    plaintext_password   = "password"
    max_user_connections = 15
    max_statement_time   = 30.5
}
`

const testAccUserConfig_resourceLimitsMariaDBUpdated = `
resource "mysql_user" "test" {
    user                 = "limited_user_mariadb"
    host                 = "%"
    plaintext_password   = "password"
    max_user_connections = 25
    max_statement_time   = 45.5
}
`

const testAccUserConfig_resourceLimitsMariaDBRemoved = `
resource "mysql_user" "test" {
    user               = "limited_user_mariadb"
    host               = "%"
    plaintext_password = "password"
}
`

const testAccUserConfig_resourceLimitsFractional = `
resource "mysql_user" "test" {
    user                 = "fractional_user"
    host                 = "%"
    plaintext_password   = "password"
    max_statement_time   = 0.01
}
`

const testAccUserConfig_resourceLimitsErrorMySQL = `
resource "mysql_user" "test" {
    user                 = "error_user"
    host                 = "%"
    plaintext_password   = "password"
    max_statement_time   = 30.0
}
`

const testAccUserConfig_resourceLimitsErrorTiDB = `
resource "mysql_user" "test" {
    user                 = "error_user_tidb"
    host                 = "%"
    plaintext_password   = "password"
    max_user_connections = 10
}
`
