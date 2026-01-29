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
	log.Printf("[DEBUG] CreateRole: roleName=%q, escaped=%q", roleName, quoteRoleName(roleName))

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

	roleName := unescapeRoleName(d.Id())
	log.Printf("[DEBUG] ReadRole: d.Id()=%q, unescaped=%q, escaped=%q", d.Id(), roleName, quoteRoleName(roleName))
	sql := fmt.Sprintf("SHOW GRANTS FOR %s", quoteRoleName(roleName))
	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		errorNumber := mysqlErrorNumber(err)
		if errorNumber == unknownUserErrCode || errorNumber == userNotFoundErrCode || errorNumber == nonExistingGrantErrCode {
			d.SetId("")
			return nil
		}
		return diag.Errorf("error reading role: %s", err)
	}
	defer rows.Close()

	if !rows.Next() {
		d.SetId("")
		return nil
	}

	d.SetId(roleName)
	d.Set("name", roleName)

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
