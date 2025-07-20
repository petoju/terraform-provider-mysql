package mysql

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUserPassword() *schema.Resource {
	return &schema.Resource{
		CreateContext: SetUserPassword,
		UpdateContext: SetUserPassword,
		ReadContext:   ReadUserPassword,
		DeleteContext: DeleteUserPassword,
		Schema: map[string]*schema.Schema{
			"user": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"host": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "localhost",
			},
			"plaintext_password": {
				Type:      schema.TypeString,
				Sensitive: true,
				Optional:  true,
			},

			"retain_old_password": {
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
	}
}

func SetUserPassword(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	uuid, err := uuid.NewV4()
	if err != nil {
		return diag.Errorf("failed getting UUID: %v", err)
	}

	var passwordStr string
	password, passOk := d.GetOk("plaintext_password")
	if !passOk {
		passwordStr = uuid.String()
		d.Set("plaintext_password", passwordStr)
	} else {
		passwordStr = password.(string)
	}

	retainPassword := d.Get("retain_old_password").(bool)
	if retainPassword {
		err := checkRetainCurrentPasswordSupport(ctx, meta)
		if err != nil {
			return diag.Errorf("cannot use retain_current_password: %v", err)
		}
	}

	stmtSQL, err := getSetPasswordStatement(ctx, meta, d.Get("user").(string), d.Get("host").(string), passwordStr, retainPassword)
	if err != nil {
		return diag.Errorf("failed getting password statement: %v", err)
	}

	// Log with password redacted
	logStmt := strings.Replace(stmtSQL, quoteString(passwordStr), "<SENSITIVE>", -1)
	log.Println("[DEBUG] Executing statement:", logStmt)

	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed executing change statement: %v", err)
	}
	user := fmt.Sprintf("%s@%s",
		d.Get("user").(string),
		d.Get("host").(string))
	d.SetId(user)
	return nil
}

func canReadPassword(ctx context.Context, meta interface{}) (bool, error) {
	serverVersion := getVersionFromMeta(ctx, meta)
	ver, _ := version.NewVersion("8.0.0")
	return serverVersion.LessThan(ver), nil
}

func ReadUserPassword(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	canRead, err := canReadPassword(ctx, meta)
	if err != nil {
		return diag.Errorf("cannot get whether we can read password: %v", err)
	}
	if !canRead {
		return nil
	}

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	results, err := db.QueryContext(ctx, `SELECT IF(PASSWORD(?) = authentication_string,'OK','FAIL') result, plugin FROM mysql.user WHERE user = ? AND host = ?`,
		d.Get("plaintext_password").(string),
		d.Get("user").(string),
		d.Get("host").(string),
	)
	if err != nil {
		// For now, we expect we are root.
		return diag.Errorf("querying auth string failed: %v", err)
	}

	for results.Next() {
		var plugin string
		var correct string
		err = results.Scan(&plugin, &correct)
		if err != nil {
			return diag.Errorf("failed reading results: %v", err)
		}

		if plugin != "mysql_native_password" {
			// We don't know whether the password is fine; it probably is.
			return nil
		}

		if correct == "FAIL" {
			d.SetId("")
			return nil
		}

		if correct == "OK" {
			return nil
		}

		return diag.Errorf("Unexpected result of query: correct: %v; plugin: %v", correct, plugin)
	}

	// User doesn't exist. Password is certainly wrong in mysql, destroy the resource.
	log.Printf("[DEBUG] User and host doesn't exist %s@%s", d.Get("user").(string), d.Get("host").(string))
	d.SetId("")
	return nil
}

func DeleteUserPassword(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// We don't need to do anything on the MySQL side here. Just need TF
	// to remove from the state file.
	return nil
}
