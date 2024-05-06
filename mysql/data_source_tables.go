package mysql

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTables() *schema.Resource {
	return &schema.Resource{
		ReadContext: ShowTables,
		Schema: map[string]*schema.Schema{
			"database": {
				Type:     schema.TypeString,
				Required: true,
			},
			"pattern": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"tables": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func ShowTables(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	database := d.Get("database").(string)
	pattern := d.Get("pattern").(string)

	sql := fmt.Sprintf("SHOW TABLES FROM %s", quoteIdentifier(database))

	if pattern != "" {
		sql += fmt.Sprintf(" LIKE '%s'", pattern)
	}

	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return diag.Errorf("failed querying for tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string

		if err := rows.Scan(&table); err != nil {
			return diag.Errorf("failed scanning MySQL rows: %v", err)
		}

		tables = append(tables, table)
	}

	if err := d.Set("tables", tables); err != nil {
		return diag.Errorf("failed setting tables field: %v", err)
	}

	d.SetId(id.UniqueId())

	return nil
}
