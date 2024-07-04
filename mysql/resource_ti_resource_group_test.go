package mysql

/*

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestTIDBResourceGroup_basic(t *testing.T) {
	varName := "rg100"
	varResourceUnits := 100
	varBurstable := true
	varPriority := "low"
	resourceName := "mysql_ti_resource_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
			// TODO: skip if not TiDB version X (7.5.2?)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccResourceGroupCheckDestroy(varName),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceGroupConfigBasic(varName, varResourceUnits),
				Check: resource.ComposeTestCheckFunc(
					testAccResourceGroupExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfigWithInstanceAndType(varName, varValue, varType, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfigWithInstanceAndType(varName, varValue, varType, varInstance),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfigWithInstanceAndType(varName, varValue, varType, varInstance),
				ExpectError: regexp.MustCompile("variable 'log.level' not found"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, "badType"),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfigWithInstanceAndType("varName", varValue, varType, varInstance),
				ExpectError: regexp.MustCompile(".*Error: bad request to:*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfigWithInstanceAndType(varName, varValue, "varType", varInstance),
				ExpectError: regexp.MustCompile(".*Error: expected type to be one of.*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfigWithInstanceAndType(varName, "varValue'varValue", varType, varInstance),
				ExpectError: regexp.MustCompile(".*Error: \"value\" is badly formatted.*"),
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
	varInstance := getGetInstance(varType, t)
	resourceName := "mysql_ti_config.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccConfigVarCheckDestroy(varName, varType),
		Steps: []resource.TestStep{
			{
				Config: testAccConfigVarConfigBasic(varName, varValue, varType),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccConfigVarConfigWithInstanceAndType(varName, varValue, varType, varInstance),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, varValue, varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config:      testAccConfigVarConfigWithInstanceAndType(varName, "varValue", varType, varInstance),
				ExpectError: regexp.MustCompile(".*Error: error setting value*"),
				Check: resource.ComposeTestCheckFunc(
					testAccConfigVarExists(varName, "varValue", varType),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
		},
	})
}

func testAccResourceGroupExists(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		getResourceGroup(varName, t)
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

type ResourceGroup struct {
	Name          string
	ResourceUnits int
	Priority      string
	Burstable     bool
	Users         []string
}

func NewResourceGroup(name string) ResourceGroup {
	return ResourceGroup{
		Name:          name,
		ResourceUnits: 2000,
		Priority:      "medium",
		Burstable:     false,
	}
}

func getResourceGroup(name string) (ResourceGroup, error) {
	rg := NewResourceGroup(name)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return ResourceGroup{}, err
	}
	query := fmt.Sprintf(`SELECT NAME, RU_PER_SEC, PRIORITY, BURSTABLE FROM information_schema.resource_groups WHERE NAME="%s";`, rg.Name)

	log.Printf("[DEBUG] SQL: %s\n", query)

	err = db.QueryRow(query).Scan(rg.Name, rg.ResourceUnits, rg.Priority, rg.Burstable)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResourceGroup{}, fmt.Errorf("error during get resource group (%s): %s", d.Id(), err)
	}

	return rg, nil
}

func testAccGetResourceGroup(varName string, db *sql.DB) (string, string, error) {
	var resType, resInstance, resName, resValue string

	configQuery := "SHOW CONFIG WHERE name = ? AND type = ?"

	stmt, err := db.Prepare(configQuery)

	if err != nil {
		return "nil", "nil", err
	}

	err = stmt.QueryRow(varName, varType).Scan(&resType, &resInstance, &resName, &resValue)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "nil", "nil", err
	}

	return resName, resValue, nil
}

func testAccResourceGroupCheckDestroy(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}

func testAccResourceGroupConfigBasic(varName string, varResourceUnits int) string {
	return fmt.Sprintf(`
resource "mysql_ti_resource_group" "test" {
		name = "%s"
		resource_units = "%s"
}
`, varName, varResourceUnits)
}

func testAccResourceGroupConfigFull(varName string, varResourceUnits int, varPriority string, varBurstable bool, users []string) string {
	return fmt.Sprintf(`
resource "mysql_ti_resource_group" "test" {
		name = "%s"
		resource_units = "%s"
		priority = "%s"
		burstable = %s
}
`, varName, varResourceUnits, varPriority, varBurstable)
}
*/
