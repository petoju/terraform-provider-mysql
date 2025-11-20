//go:build testcontainers
// +build testcontainers

package mysql

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccGlobalVar_basic_WithTestcontainers tests the mysql_global_variable resource
// Requires MySQL (not MariaDB/RDS)
// Uses shared container set up in TestMain
func TestAccGlobalVar_basic_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	varName := "max_connections"
	resourceName := "mysql_global_variable.test"
	varValue := "1"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGlobalVarCheckDestroy(varName, varValue),
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalVarConfigBasic(varName, varValue),
				Check: resource.ComposeTestCheckFunc(
					testAccGlobalVarExists(varName, varValue),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

// TestAccGlobalVar_parseBoolean_WithTestcontainers tests boolean parsing
// Requires MySQL (not MariaDB/RDS)
// Uses shared container set up in TestMain
func TestAccGlobalVar_parseBoolean_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	varName := "autocommit"
	resourceName := "mysql_global_variable.test"
	varValue := "OFF"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGlobalVarCheckDestroy(varName, varValue),
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalVarConfigBasic(varName, varValue),
				Check: resource.ComposeTestCheckFunc(
					testAccGlobalVarExists(varName, varValue),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

// Note: TestAccGlobalVar_parseString and TestAccGlobalVar_parseFloat are TiDB-specific
// and require TiDB containers, so they are not converted here.
