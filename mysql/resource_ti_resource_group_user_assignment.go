package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceTiResourceGroupUserAssignment() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateOrUpdateResourceGroupUser,
		ReadContext:   ReadResourceGroupUser,
		UpdateContext: CreateOrUpdateResourceGroupUser,
		DeleteContext: DeleteResourceGroupUser,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"user": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"resource_group": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func CreateOrUpdateResourceGroupUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// TODO: should this be the d.Id()?
	user := d.Get("user").(string)
	resourceGroup := d.Get("resource_group").(string)

	var warnLevel, warnMessage string
	var warnCode int = 0

	_, _, err = readUserFromDB(db, user)
	if err != nil {
		d.SetId("")
		return diag.Errorf(`error getting user %s`, err)
	}

	sql := fmt.Sprintf("ALTER USER `%s` RESOURCE GROUP `%s`", user, resourceGroup)
	log.Printf("[DEBUG] SQL: %s\n", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		d.SetId("")
		return diag.Errorf("error attaching user (%s) to resource group (%s): %s", user, resourceGroup, err)
	}

	// TODO: relevant?
	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		d.SetId("")
		return diag.Errorf("error setting value: %s -> %s Error: %s", user, resourceGroup, warnMessage)
	}

	d.SetId(user)
	return nil
}

func ReadResourceGroupUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var user, resourceGroup string

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	user, resourceGroup, err = readUserFromDB(db, d.Id())
	if err != nil {
		d.SetId("")
		return diag.Errorf(`error getting user %s`, err)
	}

	d.Set("user", user)
	d.Set("resourceGroup", resourceGroup)

	return nil
}

func DeleteResourceGroupUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)
	// TODO: should we re-assert that it's part of the expected resourceGroup first? I think no bc plan should read it

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	deleteQuery := fmt.Sprintf("ALTER USER `%s` RESOURCE GROUP `default`", name)
	_, err = db.Exec(deleteQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return diag.Errorf("error during drop resource group (%s): %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func readUserFromDB(db *sql.DB, name string) (string, string, error) {
	selectUsersQuery := `SELECT USER, IFNULL(JSON_EXTRACT(User_attributes, "$.resource_group"), "") as resource_group FROM mysql.user WHERE USER = ?`
	row := db.QueryRow(selectUsersQuery, name)

	var user, resourceGroup string

	err := row.Scan(&user, &resourceGroup)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", sql.ErrNoRows
	} else if err != nil {
		return "", "", fmt.Errorf(`error fetching user %e`, err)
	}

	return user, resourceGroup, nil
}
