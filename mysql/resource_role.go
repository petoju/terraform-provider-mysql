package mysql

import (
	"context"
	"database/sql"
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

	sql := fmt.Sprintf("CREATE ROLE `%s`", roleName)
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.Errorf("error creating role: %s", err)
	}

	d.SetId(roleName)

	return nil
}

// Define a struct for the role
type Role struct {
	Name               sql.NullString
	Comment            sql.NullString
	Users              sql.NullString
	GlobalPrivs        sql.NullString
	CatalogPrivs       sql.NullString
	DatabasePrivs      sql.NullString
	TablePrivs         sql.NullString
	ResourcePrivs      sql.NullString
	WorkloadGroupPrivs sql.NullString
}

func ReadRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// Execute the SHOW ROLES SQL command
	sql := "SHOW ROLES"
	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		log.Printf("[ERROR] Error executing SHOW ROLES: %s", err)
		return diag.FromErr(err)
	}
	defer rows.Close()

	// Iterate through the results to check if d.Id() is present
	roleFound := false
	for rows.Next() {
		var role Role
		if err := rows.Scan(
			&role.Name, &role.Comment, &role.Users, &role.GlobalPrivs,
			&role.CatalogPrivs, &role.DatabasePrivs, &role.TablePrivs,
			&role.ResourcePrivs, &role.WorkloadGroupPrivs); err != nil {
			log.Printf("[ERROR] Error scanning role: %s", err)
			return diag.FromErr(err)
		}
		if role.Name.String == d.Id() {
			roleFound = true
			break
		}
	}

	if !roleFound {
		log.Printf("[WARN] Role (%s) not found; removing from state", d.Id())
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

	sql := fmt.Sprintf("DROP ROLE `%s`", d.Get("name").(string))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}
