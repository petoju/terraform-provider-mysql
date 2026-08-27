package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccRole_basic(t *testing.T) {
	roleName := "tf-test-role"
	resourceName := "mysql_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQL8(t)
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

// TestAccRole_importNonExistent checks a missing role imports as absent, not as an error.
func TestAccRole_importNonExistent(t *testing.T) {
	roleName := "tf-test-role-nonexistent"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQL8(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				ResourceName:  "mysql_role.test",
				ImportState:   true,
				ImportStateId: roleName,
				Config:        testAccRoleConfigBasic(roleName),
				ExpectError:   regexp.MustCompile("Cannot import non-existent remote object"),
			},
		},
	})
}

// TestAccRole_specialCharacters covers role name quoting; account names cap at 32 chars.
func TestAccRole_specialCharacters(t *testing.T) {
	roleName := `tf-test-role'x\y`
	resourceName := "mysql_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQL8(t)
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
			{
				Config:             testAccRoleConfigBasic(roleName),
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

// TestAccRole_importUserIsRejected checks a login user cannot be imported as a role.
func TestAccRole_importUserIsRejected(t *testing.T) {
	userName := "tf-test-not-a-role"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipRds(t)
			testAccPreCheckSkipNotMySQL8(t)
		},
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRoleConfigDecoyUser(userName),
			},
			{
				// Import steps do not apply; the role block only supplies the target address.
				Config:        testAccRoleConfigDecoyUserAndRole(userName),
				ResourceName:  "mysql_role.test",
				ImportState:   true,
				ImportStateId: userName,
				ExpectError:   regexp.MustCompile("is a login user, not a role"),
			},
		},
	})
}

// TestAccClassifyAccount covers role detection, including a role later given a password,
// which only mysql.role_edges identifies.
func TestAccClassifyAccount(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC must be set for acceptance tests")
	}
	testAccPreCheckSkipRds(t)
	testAccPreCheckSkipNotMySQL8(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}

	isMariaDB, err := serverMariaDB(db)
	if err != nil {
		t.Fatalf("detecting server flavor: %v", err)
	}

	const (
		plainRole   = "tf-test-cls-role"
		grantedRole = "tf-test-cls-granted"
		pwRole      = "tf-test-cls-pwrole"
		plainUser   = "tf-test-cls-user"
		holder      = "tf-test-cls-holder"
		absent      = "tf-test-cls-absent"
	)

	cleanup := func() {
		for _, r := range []string{plainRole, grantedRole, pwRole} {
			db.ExecContext(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", quoteString(r)))
		}
		for _, u := range []string{plainUser, holder} {
			db.ExecContext(ctx, fmt.Sprintf("DROP USER IF EXISTS %s@'%%'", quoteString(u)))
		}
	}
	cleanup()
	defer cleanup()

	mustExec := func(stmt string) {
		t.Helper()
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	mustExec(fmt.Sprintf("CREATE ROLE %s", quoteString(plainRole)))
	mustExec(fmt.Sprintf("CREATE ROLE %s", quoteString(grantedRole)))
	mustExec(fmt.Sprintf("CREATE USER %s@'%%' IDENTIFIED BY 'Sekrit-Password-1'", quoteString(holder)))
	mustExec(fmt.Sprintf("CREATE USER %s@'%%' IDENTIFIED BY 'Sekrit-Password-1'", quoteString(plainUser)))
	mustExec(fmt.Sprintf("GRANT %s TO %s@'%%'", quoteString(grantedRole), quoteString(holder)))

	expected := map[string]accountKind{
		plainRole:   accountRole,
		grantedRole: accountRole,
		plainUser:   accountUser,
		holder:      accountUser,
		absent:      accountMissing,
	}

	// Roles cannot carry a password on MariaDB, where is_role settles it anyway.
	if !isMariaDB {
		mustExec(fmt.Sprintf("CREATE ROLE %s", quoteString(pwRole)))
		mustExec(fmt.Sprintf("GRANT %s TO %s@'%%'", quoteString(pwRole), quoteString(holder)))
		mustExec(fmt.Sprintf("ALTER USER %s IDENTIFIED BY 'Sekrit-Password-1' ACCOUNT UNLOCK", quoteString(pwRole)))
		expected[pwRole] = accountRole
	}

	for name, want := range expected {
		if got := classifyAccount(ctx, db, name); got != want {
			t.Errorf("classifyAccount(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestQuoteStringForRoleNames covers role name escaping; needs no database.
func TestQuoteStringForRoleNames(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		expected string
	}{
		{"plain", "developer", `'developer'`},
		{"apostrophe", "dev's", `'dev\'s'`},
		{"backslash", `dev\ops`, `'dev\\ops'`},
		{"double quote", `dev"ops`, `'dev\"ops'`},
		{"backslash and apostrophe", `a\'b`, `'a\\\'b'`},
		{"newline", "dev\nops", `'dev\nops'`},
		{"carriage return", "dev\rops", `'dev\rops'`},
		{"empty", "", `''`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteString(tt.in); got != tt.expected {
				t.Errorf("quoteString(%q) = %s, want %s", tt.in, got, tt.expected)
			}
		})
	}
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
	rows, err := db.Query(fmt.Sprintf("SHOW GRANTS FOR %s", quoteString(roleName)))
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
	// Escape backslashes first, then double quotes, for the HCL string literal.
	escaped := strings.ReplaceAll(roleName, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return fmt.Sprintf(`
resource "mysql_role" "test" {
  name = "%s"
}
`, escaped)
}

func testAccRoleConfigDecoyUser(userName string) string {
	return fmt.Sprintf(`
resource "mysql_user" "decoy" {
  user               = "%s"
  host               = "%%"
  plaintext_password = "Sekrit-Password-1"
}
`, userName)
}

func testAccRoleConfigDecoyUserAndRole(userName string) string {
	return testAccRoleConfigDecoyUser(userName) + fmt.Sprintf(`
resource "mysql_role" "test" {
  name = "%s"
}
`, userName)
}
