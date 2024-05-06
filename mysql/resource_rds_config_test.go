package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccResourceRDS(t *testing.T) {
	rName := "test"
	binlog := 24
	targetDelay := 3200
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipNotRds(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRDSConfigBasic(rName, binlog, targetDelay),
				Check: resource.ComposeTestCheckFunc(
					testAccRDSConfigExists(fmt.Sprintf("mysql_rds_config.%s", rName)),
					resource.TestCheckResourceAttr(fmt.Sprintf("mysql_rds_config.%s", rName), "binlog_retention_hours", fmt.Sprintf("%d", binlog)),
					resource.TestCheckResourceAttr(fmt.Sprintf("mysql_rds_config.%s", rName), "replication_target_delay", fmt.Sprintf("%d", targetDelay)),
				),
			},
		},
	})
}

func testAccRDSConfigBasic(rName string, binlog int, replication int) string {
	return fmt.Sprintf(`
resource "mysql_rds_config" "%s" {
                binlog_retention_hours = %d
                replication_target_delay = %d
}`, rName, binlog, replication)
}

func testAccRDSConfigExists(rn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("RDS config id not set")
		}

		return nil
	}
}

func testAccRDSCheckDestroy() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		stmtSQL := "call mysql.rds_show_configuration"

		log.Println("[DEBUG] Executing query:", stmtSQL)
		rows, err := db.QueryContext(ctx, stmtSQL)
		if err != nil {
			return err
		}

		results := make(map[string]string)
		for rows.Next() {
			var name, description string
			var value sql.NullString

			if err := rows.Scan(&name, &value, &description); err != nil {
				return fmt.Errorf("failed reading RDS config: %v", err)
			}

			if value.Valid {
				results[name] = value.String
			} else {
				results[name] = "0"
			}

			if results[name] != "0" {
				return fmt.Errorf("binlog/targetDelay still set after destroy")
			}
		}
		return nil
	}
}

func TestAccResourceRDSConfigChange(t *testing.T) {
	rName := "test_update"
	fullResourceName := fmt.Sprintf("mysql_rds_config.%s", rName)
	binlog := 24
	binlogUpdated := 48
	targetDelay := 3200
	targetDelayUpdated := 7400

	ctx := context.Background()

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipNotRds(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccRDSCheckDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testAccRDSConfigBasic(rName, binlog, targetDelay),
				Check: resource.ComposeTestCheckFunc(
					testAccRDSConfigExists(fmt.Sprintf("mysql_rds_config.%s", rName)),
				),
			},
			{
				ResourceName: fullResourceName,
				ImportState:  true,
				PreConfig: func() {
					db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
					if err != nil {
						t.Fatalf("Could not connect to MySQL instance: %v", err)
					}

					_, err = db.QueryContext(ctx, fmt.Sprintf("call mysql.rds_set_configuration('binlog retention hours', %d)", binlogUpdated))
					if err != nil {
						t.Fatalf("Failed to set binlog retention hours: %v", err)
					}

					_, err = db.QueryContext(ctx, fmt.Sprintf("call mysql.rds_set_configuration('target delay', %d)", targetDelayUpdated))
					if err != nil {
						t.Fatalf("Failed to set target delay: %v", err)
					}
				},
				Check: resource.ComposeTestCheckFunc(
					testAccRDSConfigExists(fmt.Sprintf("mysql_rds_config.%s", rName)),
					testAccRDSCheckFull(fullResourceName, binlogUpdated, targetDelayUpdated),
				),
			},
		},
	})
}

func testAccRDSCheckFull(rn string, binlogUpdated, targetDelayUpdated int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("RDS config id not set")
		}

		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		stmtSQL := "call mysql.rds_show_configuration"

		rows, err := db.QueryContext(ctx, stmtSQL)
		if err != nil {
			return err
		}

		results := make(map[string]string)
		for rows.Next() {
			var name, description string
			var value sql.NullString

			if err := rows.Scan(&name, &value, &description); err != nil {
				return fmt.Errorf("failed reading RDS config: %v", err)
			}

			if value.Valid {
				results[name] = value.String
			}
		}

		if len(results["binlog retention hours"]) == 0 || results["binlog retention hours"] == "NULL" {
			results["binlog retention hours"] = "0"
		}

		binlogRetentionPeriod, err := strconv.Atoi(results["binlog retention hours"])
		if err != nil {
			return fmt.Errorf("failed reading binlog retention RDS config: %v", err)
		}

		if len(results["target delay"]) == 0 {
			results["target delay"] = "0"
		}

		replicationTargetDelay, err := strconv.Atoi(results["target delay"])
		if err != nil {
			return fmt.Errorf("failed reading target delay RDS config: %v", err)
		}

		if binlogRetentionPeriod != binlogUpdated {
			return fmt.Errorf("binlog retention should be %d, not %d", binlogUpdated, binlogRetentionPeriod)
		}

		if replicationTargetDelay != targetDelayUpdated {
			return fmt.Errorf("target delay should be %d, not %d", targetDelayUpdated, replicationTargetDelay)
		}

		return nil
	}
}
