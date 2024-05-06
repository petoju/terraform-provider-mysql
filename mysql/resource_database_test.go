package mysql

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccDatabase(t *testing.T) {
	dbName := "terraform_acceptance_test"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipTiDB(t) },
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

func TestAccDatabase_collationChange(t *testing.T) {
	dbName := "terraform_acceptance_test"

	charset1 := "latin1"
	charset2 := "utf8mb4"
	collation1 := "latin1_bin"
	collation2 := "utf8mb4_general_ci"

	resourceName := "mysql_database.test"
	ctx := context.Background()

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipTiDB(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDatabaseCheckDestroy(dbName),
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseConfigFull(dbName, charset1, collation1),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull("mysql_database.test", dbName, charset1, collation1),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				PreConfig: func() {
					db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
					if err != nil {
						return
					}

					db.Exec(fmt.Sprintf("ALTER DATABASE %s CHARACTER SET %s COLLATE %s", dbName, charset2, collation2))
				},
				Config: testAccDatabaseConfigFull(dbName, charset1, collation1),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull(resourceName, dbName, charset1, collation1),
				),
			},
		},
	})
}

func testAccDatabaseCheckBasic(rn string, name string) resource.TestCheckFunc {
	return testAccDatabaseCheckFull(rn, name, "utf8mb4", "utf8mb4_bin")
}

func testAccDatabaseCheckFull(rn string, name string, charset string, collation string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("database id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		var _name, createSQL string
		err = db.QueryRow(fmt.Sprintf("SHOW CREATE DATABASE %s", name)).Scan(&_name, &createSQL)
		if err != nil {
			return fmt.Errorf("error reading database: %s", err)
		}

		if strings.Index(createSQL, fmt.Sprintf("CHARACTER SET %s", charset)) == -1 {
			return fmt.Errorf("database default charset isn't %s", charset)
		}
		if strings.Index(createSQL, fmt.Sprintf("COLLATE %s", collation)) == -1 {
			return fmt.Errorf("database default collation isn't %s", collation)
		}

		return nil
	}
}

func testAccDatabaseCheckDestroy(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		var _name, createSQL string
		err = db.QueryRow(fmt.Sprintf("SHOW CREATE DATABASE %s", name)).Scan(&_name, &createSQL)
		if err == nil {
			return fmt.Errorf("database still exists after destroy")
		}

		if mysqlErrorNumber(err) == unknownDatabaseErrCode {
			return nil
		}

		return fmt.Errorf("got unexpected error: %s", err)
	}
}

func testAccDatabaseConfigBasic(name string) string {
	return testAccDatabaseConfigFull(name, "utf8mb4", "utf8mb4_bin")
}

func testAccDatabaseConfigFull(name string, charset string, collation string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
    name = "%s"
    default_character_set = "%s"
    default_collation = "%s"
}`, name, charset, collation)
}
