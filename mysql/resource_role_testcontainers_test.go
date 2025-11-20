//go:build testcontainers
// +build testcontainers

package mysql

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccRole_basic_WithTestcontainers tests the mysql_role resource
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain (MySQL 8.0 required for roles)
func TestAccRole_basic_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	roleName := "tf-test-role"
	resourceName := "mysql_role.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
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
		},
	})
}
