package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestTIDBResourceGroup_basic_to_full(t *testing.T) {
	varName := "rg100"
	varResourceUnits := 100
	varNewResourceUnits := 1000
	varQueryLimit := ""
	varNewQueryLimit := "EXEC_ELAPSED='15s', ACTION=COOLDOWN, WATCH=SIMILAR DURATION='10m0s'"
	varBurstable := true
	varPriority := "low"
	resourceName := "mysql_ti_resource_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
			testAccPreCheckSkipNotTiDBVersionMin(t, ResourceGroupTiDBMinVersion)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccResourceGroupCheckDestroy(varName),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceGroupConfigBasic(varName, varResourceUnits, varQueryLimit),
				Check: resource.ComposeTestCheckFunc(
					testAccResourceGroupExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
					resource.TestCheckResourceAttr(resourceName, "query_limit", varQueryLimit),
				),
			},
			{
				Config: testAccResourceGroupConfigFull(varName, varNewResourceUnits, varNewQueryLimit, varBurstable, varPriority),
				Check: resource.ComposeTestCheckFunc(
					testAccResourceGroupExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
					resource.TestCheckResourceAttr(resourceName, "query_limit", varNewQueryLimit),
					resource.TestCheckResourceAttr(resourceName, "burstable", fmt.Sprintf("%t", varBurstable)),
					resource.TestCheckResourceAttr(resourceName, "priority", varPriority),
				),
			},
		},
	})
}

func TestTIDBResourceGroup_full_to_basic(t *testing.T) {
	varName := "rg100"
	varResourceUnits := 1000
	varNewResourceUnits := 100
	varQueryLimit := "EXEC_ELAPSED='15s', ACTION=COOLDOWN, WATCH=SIMILAR DURATION='10m0s'"
	varNewQueryLimit := ""
	varBurstable := true
	varPriority := "low"
	resourceName := "mysql_ti_resource_group.test"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
			testAccPreCheckSkipNotTiDBVersionMin(t, ResourceGroupTiDBMinVersion)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccResourceGroupCheckDestroy(varName),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceGroupConfigFull(varName, varResourceUnits, varQueryLimit, varBurstable, varPriority),
				Check: resource.ComposeTestCheckFunc(
					testAccResourceGroupExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
					resource.TestCheckResourceAttr(resourceName, "query_limit", varQueryLimit),
					resource.TestCheckResourceAttr(resourceName, "burstable", fmt.Sprintf("%t", varBurstable)),
					resource.TestCheckResourceAttr(resourceName, "priority", varPriority),
				),
			},
			{
				Config: testAccResourceGroupConfigBasic(varName, varNewResourceUnits, varNewQueryLimit),
				Check: resource.ComposeTestCheckFunc(
					testAccResourceGroupExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
					resource.TestCheckResourceAttr(resourceName, "query_limit", varNewQueryLimit),
				),
			},
		},
	})
}

func testAccResourceGroupExists(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rg, err := getResourceGroup(varName)
		if err != nil {
			return err
		}

		if rg == nil {
			return fmt.Errorf("resource group (%s) does not exist", varName)
		}

		return nil
	}
}

func NewResourceGroup(name string) *ResourceGroup {
	return &ResourceGroup{
		Name:          name,
		ResourceUnits: 2000,
		Priority:      "medium",
		Burstable:     false,
		QueryLimit:    "EXEC_ELAPSED='15s', ACTION=COOLDOWN, WATCH=SIMILAR DURATION='10m0s'",
	}
}

func getResourceGroup(name string) (*ResourceGroup, error) {
	rg := NewResourceGroup(name)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT NAME, RU_PER_SEC, LOWER(PRIORITY), BURSTABLE = 'YES' as BURSTABLE, IFNULL(QUERY_LIMIT, "") FROM information_schema.resource_groups WHERE NAME="%s";`, rg.Name)

	log.Printf("[DEBUG] SQL: %s\n", query)

	err = db.QueryRow(query).Scan(&rg.Name, &rg.ResourceUnits, &rg.Priority, &rg.Burstable, &rg.QueryLimit)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("error during get resource group (%s): %s", rg.Name, err)
	}

	return rg, nil
}

func testAccResourceGroupCheckDestroy(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}

func testAccResourceGroupConfigBasic(varName string, varResourceUnits int, varQueryLimit string) string {
	return fmt.Sprintf(`
resource "mysql_ti_resource_group" "test" {
		name = "%s"
		resource_units = %d
		query_limit = "%s"
}
`, varName, varResourceUnits, varQueryLimit)
}

func testAccResourceGroupConfigFull(varName string, varResourceUnits int, varQueryLimit string, varBurstable bool, varPriority string) string {
	return fmt.Sprintf(`
resource "mysql_ti_resource_group" "test" {
		name = "%s"
		resource_units = %d
		priority = "%s"
		burstable = %t
		query_limit = "%s"
}
`, varName, varResourceUnits, varPriority, varBurstable, varQueryLimit)
}
