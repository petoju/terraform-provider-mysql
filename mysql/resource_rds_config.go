package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// stable non-empty ID
const mysqlRdsConfigId = "1223234548"

func resourceRDSConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateRDSConfig,
		UpdateContext: UpdateRDSConfig,
		ReadContext:   ReadRDSConfig,
		DeleteContext: DeleteRDSConfig,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"binlog_retention_hours": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Sets the number of hours to retain binary log files",
			},
			"replication_target_delay": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Sets the number of seconds to delay replication from source database instance to the read replica",
			},
		},
	}
}

func CreateRDSConfig(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, stmtSQL := range RDSConfigSQL(d) {
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		_, err = db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed running SQL to set RDS Config: %v", err)
		}
	}

	d.SetId(mysqlRdsConfigId)

	return nil
}

func UpdateRDSConfig(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, stmtSQL := range RDSConfigSQL(d) {
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		_, err = db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed updating RDS config: %v", err)
		}
	}

	return nil
}

func ReadRDSConfig(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL := "call mysql.rds_show_configuration"

	log.Println("[DEBUG] Executing query:", stmtSQL)
	rows, err := db.QueryContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("Error reading RDS config from DB: %v", err)
	}

	results := make(map[string]string)
	for rows.Next() {
		var name, description string
		var value sql.NullString

		if err := rows.Scan(&name, &value, &description); err != nil {
			return diag.Errorf("failed validating RDS config: %v", err)
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
		return diag.Errorf("failed reading binlog retention hours in RDS config: %v", err)
	}

	if len(results["target delay"]) == 0 {
		results["target delay"] = "0"
	}

	replicationTargetDelay, err := strconv.Atoi(results["target delay"])
	if err != nil {
		return diag.Errorf("failed reading target delay in RDS config: %v", err)
	}

	d.Set("replication_target_delay", replicationTargetDelay)
	d.Set("binlog_retention_hours", binlogRetentionPeriod)

	return nil
}

func DeleteRDSConfig(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtsSQL := []string{"call mysql.rds_set_configuration('binlog retention hours', NULL)", "call mysql.rds_set_configuration('target delay', 0)"}
	for _, stmtSQL := range stmtsSQL {
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		_, err = db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed unsetting RDS config: %v", err)
		}
	}

	d.SetId("")
	return nil
}

func RDSConfigSQL(d *schema.ResourceData) []string {
	result := []string{}
	if d.Get("binlog_retention_hours") != nil {
		retentionPeriod := strconv.Itoa(d.Get("binlog_retention_hours").(int))
		if retentionPeriod == "0" {
			retentionPeriod = "NULL"
		}
		result = append(result, fmt.Sprintf("call mysql.rds_set_configuration('binlog retention hours', %s)", retentionPeriod))
	}

	if d.Get("replication_target_delay") != nil {
		targetDelay := d.Get("replication_target_delay")
		result = append(result, fmt.Sprintf("call mysql.rds_set_configuration('target delay', %v)", targetDelay))
	}

	return result
}
