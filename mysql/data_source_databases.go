package mysql

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDatabases() *schema.Resource {
	return &schema.Resource{
		ReadContext: ShowDatabases,
		Schema: map[string]*schema.Schema{
			"pattern": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"databases": {
				Type:     schema.TypeList,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func ShowDatabases(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	pattern := d.Get("pattern").(string)

	sql := fmt.Sprint("SHOW DATABASES")

	if pattern != "" {
		sql += fmt.Sprintf(" LIKE '%s'", pattern)
	}

	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return diag.Errorf("failed querying for databases: %v", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string

		if err := rows.Scan(&database); err != nil {
			return diag.Errorf("failed scanning MySQL rows: %v", err)
		}

		databases = append(databases, database)
	}

	if err := d.Set("databases", databases); err != nil {
		return diag.Errorf("failed setting databases field: %v", err)
	}

	d.SetId(id.UniqueId())

	return nil
}
