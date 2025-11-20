//go:build testcontainers
// +build testcontainers

package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TestAccDatabase_WithTestcontainers is a proof of concept test
// using Testcontainers instead of Makefile + Docker
// Uses shared container set up in TestMain
func TestAccDatabase_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := "terraform_acceptance_test"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDatabaseCheckDestroy(dbName),
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseConfigBasic(dbName),
				Check: testAccDatabaseCheckBasic(
					"mysql_database.test", dbName,
				),
			},
			{
				Config:            testAccDatabaseConfigBasic(dbName),
				ResourceName:      "mysql_database.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     dbName,
			},
		},
	})
}

// TestAccDatabase_collationChange_WithTestcontainers tests collation changes
// Uses shared container set up in TestMain
func TestAccDatabase_collationChange_WithTestcontainers(t *testing.T) {
	// Use shared container set up in TestMain
	_ = getSharedMySQLContainer(t, "mysql:8.0")

	dbName := "terraform_acceptance_test"

	charset1 := "latin1"
	charset2 := "utf8mb4"
	collation1 := "latin1_bin"
	collation2 := "utf8mb4_general_ci"

	resourceName := "mysql_database.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDatabaseCheckDestroy(dbName),
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseConfigFull(dbName, charset1, collation1, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull("mysql_database.test", dbName, charset1, collation1, ""),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				PreConfig: func() {
					ctx := context.Background()
					db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
					if err != nil {
						return
					}

					db.Exec(fmt.Sprintf("ALTER DATABASE %s CHARACTER SET %s COLLATE %s", dbName, charset2, collation2))
				},
				Config: testAccDatabaseConfigFull(dbName, charset1, collation1, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull(resourceName, dbName, charset1, collation1, ""),
				),
			},
		},
	})
}
