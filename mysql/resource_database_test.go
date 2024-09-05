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
		PreCheck:          func() {},
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
		PreCheck:          func() {},
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

func TestAccDatabase_placementPolicyChange(t *testing.T) {
	dbName := "terraform_acceptance_test"

	charset1 := "latin1"
	collation1 := "latin1_bin"
	placementPolicy1 := "test_policy"
	placementPolicy2 := "test_policy_v2"
	placementPolicyResourceName := "mysql_ti_placement_policy.test.name"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheckSkipNotTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccDatabaseCheckDestroy(dbName),
		Steps: []resource.TestStep{
			{
				Config: testAccDatabaseAndPlacementPolicy(dbName, charset1, collation1, placementPolicy1, placementPolicyResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull("mysql_database.test", dbName, charset1, collation1, placementPolicy1),
				),
			},
			{
				Config: testAccDatabaseAndPlacementPolicy(dbName, charset1, collation1, placementPolicy1, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull("mysql_database.test", dbName, charset1, collation1, ""),
				),
			},
			{
				Config: testAccDatabaseAndPlacementPolicy(dbName, charset1, collation1, placementPolicy2, placementPolicyResourceName),
				Check: resource.ComposeTestCheckFunc(
					testAccDatabaseCheckFull("mysql_database.test", dbName, charset1, collation1, placementPolicy2),
				),
			},
		},
	})
}

func testAccDatabaseCheckBasic(rn string, name string) resource.TestCheckFunc {
	return testAccDatabaseCheckFull(rn, name, "utf8mb4", "utf8mb4_bin", "")
}

func testAccDatabaseCheckFull(rn string, name string, charset string, collation string, placementPolicy string) resource.TestCheckFunc {
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

		if !strings.Contains(createSQL, fmt.Sprintf("CHARACTER SET %s", charset)) {
			return fmt.Errorf("database default charset isn't %s", charset)
		}
		// TiDB does not include the COLLATE reference in `SHOW CREATE DATABASE`
		// so perform a lookup based on the charset to find default collation
		if !strings.Contains(createSQL, fmt.Sprintf("COLLATE %s", collation)) {
			sql := `SELECT COLLATION_NAME FROM INFORMATION_SCHEMA.COLLATIONS WHERE IS_DEFAULT = 'Yes' AND CHARACTER_SET_NAME = ?;`
			var fetchedCollation string
			err = db.QueryRow(sql, charset).Scan(&fetchedCollation)
			if err != nil {
				return fmt.Errorf("database default collation expected %s vs actual %s with error: %e", collation, fetchedCollation, err)
			}
			if fetchedCollation != collation {
				return fmt.Errorf("database default collation expected %s vs actual %s", collation, fetchedCollation)
			}
		}

		if !strings.Contains(createSQL, fmt.Sprintf("PLACEMENT POLICY=`%s`", placementPolicy)) && placementPolicy != "" {
			return fmt.Errorf("placement policy expected %s", placementPolicy)
		} else if strings.Contains(createSQL, "PLACEMENT POLICY=") && placementPolicy == "" {
			return fmt.Errorf("placement policy expected to be empty")
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
	return testAccDatabaseConfigFull(name, "utf8mb4", "utf8mb4_bin", "")
}

func testAccDatabaseConfigFull(name string, charset string, collation string, placementPolicy string) string {
	placementPolicyConfig := ""
	if placementPolicy != "" {
		placementPolicyConfig = fmt.Sprintf(`placement_policy = %s`, placementPolicy)
	}

	return fmt.Sprintf(`
resource "mysql_database" "test" {
    name = "%s"
    default_character_set = "%s"
    default_collation = "%s"
    %s
}`, name, charset, collation, placementPolicyConfig)
}

func testAccDatabaseAndPlacementPolicy(name string, charset string, collation string, placementPolicy string, databasePlacementPolicy string) string {
	return fmt.Sprintf(
		"%s\n%s",
		testAccPlacementPolicyConfigBasic(placementPolicy),
		testAccDatabaseConfigFull(name, charset, collation, databasePlacementPolicy),
	)
}
