package mysql

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestPdConfigVar_basic(t *testing.T) {
	varName := "log.level"
	varValue := "warn"
	varType := "pd"
	varInstance := ""
	resourceName := "mysql_ti_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccConfigVarCheckDestroy(varName, varType),
		Steps: []resource.TestStep{
			{
				Config: testAccConfigVarConfig_basic(varName, varValue, varType),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfig_withInstanceAndType(varName, varValue, varType, varInstance),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfig_withInstanceAndType(varName, varValue, varType, getGetInstance(varType, t)),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfig_withInstanceAndType(varName, varValue, varType, getGetInstance(varType, t)),
				ExpectError: regexp.MustCompile("variable 'log.level' not found"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, "badType"),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfig_withInstanceAndType("varName", varValue, varType, getGetInstance(varType, t)),
				ExpectError: regexp.MustCompile(".*Error: bad request to:*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfig_withInstanceAndType(varName, varValue, "varType", getGetInstance(varType, t)),
				ExpectError: regexp.MustCompile(".*Error: expected type to be one of.*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

func TestTiKvConfigVar_basic(t *testing.T) {
	varName := "split.qps-threshold"
	varValue := "1000"
	varType := "tikv"
	varInstance := ""
	resourceName := "mysql_ti_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccConfigVarCheckDestroy(varName, varType),
		Steps: []resource.TestStep{
			{
				Config: testAccConfigVarConfig_basic(varName, varValue, varType),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfig_withInstanceAndType(varName, varValue, varType, varInstance),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfig_withInstanceAndType(varName, "varValue", varType, varInstance),
				ExpectError: regexp.MustCompile(".*Error: error setting value*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, "varValue", varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

func testAccConfigVarExists(varName string, varValue string, varType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		resName, resValue, err := testAccGetConfigVar(varName, varType, db)

		if err != nil {
			return err
		}

		if resValue == varValue {
			return nil
		}

		return fmt.Errorf("variable '%s' not found. resName: %s, resValue: %s, err: %s", varName, resName, resValue, err)
	}
}

func getGetInstance(varType string, t *testing.T) string {
	var resInstanceType, resInstance, resName, resValue string
	match, _ := regexp.MatchString("^(mysql:).*", os.Getenv("DB"))
	if match {
		t.Skip("Skip on MySQL")
	}

	db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return err.Error()
	}
	configQuery := "SHOW CONFIG WHERE type = ? AND name = 'log.level'"

	stmt, err := db.Prepare(configQuery)

	if err != nil {
		return err.Error()
	}

	err = stmt.QueryRow(varType).Scan(&resInstanceType, &resInstance, &resName, &resValue)

	if err != nil && err != sql.ErrNoRows {
		return err.Error()
	}

	return resInstance
}

func testAccGetConfigVar(varName string, varType string, db *sql.DB) (string, string, error) {
	var resType, resInstance, resName, resValue string

	configQuery := "SHOW CONFIG WHERE name = ? AND type = ?"

	stmt, err := db.Prepare(configQuery)

	if err != nil {
		return "nil", "nil", err
	}

	err = stmt.QueryRow(varName, varType).Scan(&resType, &resInstance, &resName, &resValue)

	if err != nil && err != sql.ErrNoRows {
		return "nil", "nil", err
	}

	return resName, resValue, nil
}

func testAccConfigVarCheckDestroy(varName string, varType string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}

func testAccConfigVarConfig_basic(varName string, varValue string, varType string) string {
	return fmt.Sprintf(`
resource "mysql_ti_config" "test" {
  name = "%s"
	value = "%s"
	type = "%s"
}
`, varName, varValue, varType)
}

func testAccConfigVarConfig_withInstanceAndType(varName string, varValue string, varType string, varInstance string) string {
	return fmt.Sprintf(`
resource "mysql_ti_config" "test" {
  name = "%s"
	value = "%s"
	type = "%s"
	instance = "%s"
}
`, varName, varValue, varType, varInstance)
}
