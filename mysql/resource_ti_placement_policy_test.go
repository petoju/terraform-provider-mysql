package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestTIDBPlacementPolicy_basic(t *testing.T) {
	resourceName := "mysql_ti_placement_policy.test"
	varName := "test_policy"
	varPrimaryRegion := ""
	varRegions := `[]`
	varConstraints := `["+key=value"]`

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipNotTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccPlacementPolicyCheckDestroy(varName),
		Steps: []resource.TestStep{
			{
				Config: testAccPlacementPolicyConfigBasic(varName),
				Check: resource.ComposeTestCheckFunc(
					testAccPlacementPolicyExists(varName),
					resource.TestCheckResourceAttr(resourceName, "name", varName),
				),
			},
			{
				Config: testAccPlacementPolicyConfigFull(varName, varPrimaryRegion, varRegions, varConstraints),
				Check: resource.ComposeTestCheckFunc(
					testAccPlacementPolicyExists(varName),
					resource.TestCheckResourceAttr(resourceName, "primary_region", varPrimaryRegion),
					resource.TestCheckResourceAttr(resourceName, "constraints.0", "+key=value"),
				),
			},
		},
	})
}

func testAccPlacementPolicyExists(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rg, err := getPlacementPolicy(varName)
		if err != nil {
			return err
		}

		if rg == nil {
			return fmt.Errorf("placement policy (%s) does not exist", varName)
		}

		return nil
	}
}

func getPlacementPolicy(name string) (*PlacementPolicy, error) {
	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return nil, err
	}

	return getPlacementPolicyFromDB(db, name)
}

func testAccPlacementPolicyCheckDestroy(varName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		return nil
	}
}

func testAccPlacementPolicyConfigBasic(varName string) string {
	return fmt.Sprintf(`
resource "mysql_ti_placement_policy" "test" {
  name = "%s"
}
`, varName)
}

func testAccPlacementPolicyConfigFull(varName string, varPrimaryRegion string, varRegions string, varConstraints string) string {
	return fmt.Sprintf(`
resource "mysql_ti_placement_policy" "test" {
		name = "%s"
		primary_region = "%s"
		regions = %s
		constraints = %s
}
`, varName, varPrimaryRegion, varRegions, varConstraints)
}
