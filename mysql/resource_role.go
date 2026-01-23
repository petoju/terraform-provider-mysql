package mysql

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRole() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateRole,
		ReadContext:   ReadRole,
		DeleteContext: DeleteRole,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func CreateRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	roleName := d.Get("name").(string)

	sql := fmt.Sprintf("CREATE ROLE %s", quoteRoleName(roleName))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.Errorf("error creating role: %s", err)
	}

	d.SetId(roleName)

	return nil
}

func ReadRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	sql := fmt.Sprintf("SHOW GRANTS FOR %s", quoteRoleName(d.Id()))
	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		errorNumber := mysqlErrorNumber(err)
		if errorNumber == 1133 || errorNumber == 1396 || errorNumber == 1141 {
			log.Printf("[WARN] Role %s does not exist, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	defer rows.Close()

	if !rows.Next() {
		log.Printf("[WARN] Role %s does not exist (no grants returned), removing from state", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("name", d.Id())

	return nil
}

func DeleteRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	sql := fmt.Sprintf("DROP ROLE %s", quoteRoleName(d.Get("name").(string)))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
