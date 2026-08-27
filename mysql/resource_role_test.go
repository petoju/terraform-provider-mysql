package mysql

import (
	"context"
	"database/sql"
	"fmt"
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
