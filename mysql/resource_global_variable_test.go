package mysql

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccGlobalVar_basic(t *testing.T) {
	varName := "max_connections"
	varValue := 1
	resourceName := "mysql_global_variable.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPreCheckSkipMariaDB(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGlobalVarCheckDestroy(varName, fmt.Sprint(varValue)),
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalVarConfig_intValue(varName, varValue),
				Check: resource.ComposeTestCheckFunc(
					testAccGlobalVarExists(varName, fmt.Sprint(varValue)),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

func TestAccGlobalVar_variableTiDBTests(t *testing.T) {
	varName := "tidb_auto_analyze_end_time"
	varValue := "07:00 +0300"
	resourceName := "mysql_global_variable.test"

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t); testAccPreCheckSkipMariaDB(t); testAccPreCheckSkipNotTiDB(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGlobalVarCheckDestroy(varName, varValue),
		Steps: []resource.TestStep{
			{
				Config: testAccGlobalVarConfig_stringValue(varName, varValue),
				Check: resource.ComposeTestCheckFunc(
					testAccGlobalVarExists(varName, varValue),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

func testAccGlobalVarExists(varName, varExpected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		varReturned, err := testAccGetGlobalVar(varName, db)
		if err != nil {
			return err
		}

		if varReturned == varExpected {
			return nil
		}

		return fmt.Errorf("variable '%s' not found", varName)
	}
}

func testAccGetGlobalVar(varName string, db *sql.DB) (string, error) {
	stmt, err := db.Prepare("SHOW GLOBAL VARIABLES WHERE VARIABLE_NAME = ?")
	if err != nil {
		return "", err
	}

	var name, value string
	err = stmt.QueryRow(varName).Scan(&name, &value)

	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	return value, nil
}

func testAccGlobalVarCheckDestroy(varName, varExpected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		varReturned, _ := testAccGetGlobalVar(varName, db)
		if varReturned == varExpected {
			return fmt.Errorf("Global variable '%s' still has non default value", varName)
		}

		return nil
	}
}

func testAccGlobalVarConfig_intValue(varName string, varValue int) string {
	return fmt.Sprintf(`
resource "mysql_global_variable" "test" {
  name = "%s"
	value = %d
}
`, varName, varValue)
}

func testAccGlobalVarConfig_stringValue(varName, varValue string) string {
	return fmt.Sprintf(`
resource "mysql_global_variable" "test" {
  name = "%s"
	value = "%s"
}
`, varName, varValue)
}
