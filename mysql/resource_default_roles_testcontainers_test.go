//go:build testcontainers
// +build testcontainers

package mysql

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccDefaultRoles_basic_WithTestcontainers tests the mysql_default_roles resource
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain (MySQL 8.0 required for default roles)
func TestAccDefaultRoles_basic_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDefaultRolesCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccDefaultRolesBasic,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test", "role1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.0", "role1"),
				),
			},
			{
				Config: testAccDefaultRolesMultiple,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test", "role1", "role2"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "2"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.0", "role1"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.1", "role2"),
				),
			},
			{
				Config: testAccDefaultRolesNone,
				Check: resource.ComposeTestCheckFunc(
					testAccDefaultRoles("mysql_default_roles.test"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "0"),
				),
			},
			{
				Config:            testAccDefaultRolesBasic,
				ResourceName:      "mysql_default_roles.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v", "jdoe", "%"),
			},
			{
				Config:            testAccDefaultRolesMultiple,
				ResourceName:      "mysql_default_roles.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     fmt.Sprintf("%v@%v", "jdoe", "%"),
			},
		},
	})
}
