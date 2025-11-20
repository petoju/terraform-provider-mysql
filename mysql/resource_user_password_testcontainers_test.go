//go:build testcontainers
// +build testcontainers

package mysql

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccUserPassword_basic_WithTestcontainers tests the mysql_user_password resource
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain
func TestAccUserPassword_basic_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccUserCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccUserPasswordConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccUserExists("mysql_user.test"),
					resource.TestCheckResourceAttr("mysql_user_password.test", "user", "jdoe"),
					resource.TestCheckResourceAttrSet("mysql_user_password.test", "plaintext_password"),
				),
			},
		},
	})
}
