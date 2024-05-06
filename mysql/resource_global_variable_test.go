package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccGlobalVar_basic(t *testing.T) {
	varName := "max_connections"
	resourceName := "mysql_global_variable.test"
	varValue := "1"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t); testAccPreCheckSkipMariaDB(t); testAccPreCheckSkipRds(t) },
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

func TestAccGlobalVar_parseString(t *testing.T) {
	varName := "tidb_auto_analyze_end_time"
	resourceName := "mysql_global_variable.test"
	varValue := "07:00 +0300"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotTiDB(t)
			testAccPreCheckSkipRds(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccGlobalVarCheckDestroy(varName, varValue),
		Steps: []resource.TestStep{
			{
				Config:      testAccGlobalVarConfigBasic(varName, "varValue'varValue"),
				ExpectError: regexp.MustCompile(".*is badly formatted.*"),
			},
			{
				Config: testAccGlobalVarConfigBasic("tidb_auto_analyze_ratio", "0.4"),
				Check: resource.ComposeTestCheckFunc(
					testAccGlobalVarExists("tidb_auto_analyze_ratio", "0.4"),
					resource.TestCheckResourceAttr(resourceName, "name", "tidb_auto_analyze_ratio"),
				),
			},
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

func TestAccGlobalVar_parseFloat(t *testing.T) {
	varName := "tidb_auto_analyze_ratio"
	resourceName := "mysql_global_variable.test"
	varValue := "0.4"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotTiDB(t)
			testAccPreCheckSkipRds(t)
		},
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

func TestAccGlobalVar_parseBoolean(t *testing.T) {
	varName := "autocommit"
	resourceName := "mysql_global_variable.test"
	varValue := "OFF"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipMariaDB(t)
			testAccPreCheckSkipNotTiDB(t)
			testAccPreCheckSkipRds(t)
		},
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

func testAccGlobalVarExists(varName, varExpected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		res, err := testAccGetGlobalVar(varName, db)

		if err != nil {
			return err
		}

		if res == varExpected {
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

	var name string
	var value string
	err = stmt.QueryRow(varName).Scan(&name, &value)

	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	return value, nil
}

func testAccGlobalVarCheckDestroy(varName, varExpected string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		res, _ := testAccGetGlobalVar(varName, db)
		if res == varExpected {
			return fmt.Errorf("global variable '%s' still has non default value", varName)
		}

		return nil
	}
}

func testAccGlobalVarConfigBasic(varName, varValue string) string {
	return fmt.Sprintf(`
resource "mysql_global_variable" "test" {
  name = "%s"
	value = "%s"
}
`, varName, varValue)
}
