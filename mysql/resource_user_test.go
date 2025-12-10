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
