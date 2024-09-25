package mysql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateUser,
		UpdateContext: UpdateUser,
		ReadContext:   ReadUser,
		DeleteContext: DeleteUser,
		Importer: &schema.ResourceImporter{
			StateContext: ImportUser,
		},

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
				Optional:  true,
				Sensitive: true,
				StateFunc: hashSum,
			},

			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"plaintext_password"},
				Sensitive:     true,
				Deprecated:    "Please use plaintext_password instead",
			},

			"auth_plugin": {
				Type:             schema.TypeString,
				Optional:         true,
				ForceNew:         true,
				DiffSuppressFunc: NewEmptyStringSuppressFunc,
				ConflictsWith:    []string{"password"},
			},

			"aad_identity": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
							Default:  "user",
							ValidateFunc: validation.StringInSlice([]string{
								"user",
								"group",
								"service_principal",
							}, false),
						},
						"identity": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},

			"auth_string_hashed": {
				Type:             schema.TypeString,
				Optional:         true,
				Sensitive:        true,
				DiffSuppressFunc: NewEmptyStringSuppressFunc,
				ConflictsWith:    []string{"plaintext_password", "password"},
			},

			"tls_option": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "NONE",
			},

			"retain_old_password": {
				Type:     schema.TypeBool,
				Optional: true,
			},
		},
	}
}

func checkRetainCurrentPasswordSupport(ctx context.Context, meta interface{}) error {
	ver, _ := version.NewVersion("8.0.14")
	if getVersionFromMeta(ctx, meta).LessThan(ver) {
		return errors.New("MySQL version must be at least 8.0.14")
	}
	return nil
}

func CreateUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	var authStm string
	var auth string
	var createObj = "USER"

	if v, ok := d.GetOk("auth_plugin"); ok {
		auth = v.(string)
	}

	if len(auth) > 0 {
		if auth == "aad_auth" {
			// aad_auth is plugin but Microsoft uses another statement to create this kind of users
			createObj = "AADUSER"
			if _, ok := d.GetOk("aad_identity"); !ok {
				return diag.Errorf("aad_identity is required for aad_auth")
			}
		} else if auth == "AWSAuthenticationPlugin" {
			authStm = " IDENTIFIED WITH AWSAuthenticationPlugin as 'RDS'"
		} else {
			// mysql_no_login, auth_pam, ...
			authStm = " IDENTIFIED WITH " + auth
		}
	}
	if v, ok := d.GetOk("auth_string_hashed"); ok {
		hashed := v.(string)
		if hashed != "" {
			if authStm == "" {
				return diag.Errorf("auth_string_hashed is not supported for auth plugin %s", auth)
			}
			authStm = fmt.Sprintf("%s AS '%s'", authStm, hashed)
		}
	}

	var stmtSQL string

	if createObj == "AADUSER" {
		var aadIdentity = d.Get("aad_identity").(*schema.Set).List()[0].(map[string]interface{})

		if aadIdentity["type"].(string) == "service_principal" {
			// CREATE AADUSER 'mysqlProtocolLoginName"@"mysqlHostRestriction' IDENTIFIED BY 'identityId'
			stmtSQL = fmt.Sprintf("CREATE AADUSER '%s'@'%s' IDENTIFIED BY '%s'",
				d.Get("user").(string),
				d.Get("host").(string),
				aadIdentity["identity"].(string))
		} else {
			// CREATE AADUSER 'identityName"@"mysqlHostRestriction' AS 'mysqlProtocolLoginName'
			stmtSQL = fmt.Sprintf("CREATE AADUSER '%s'@'%s' AS '%s'",
				aadIdentity["identity"].(string),
				d.Get("host").(string),
				d.Get("user").(string))
		}
	} else {
		stmtSQL = fmt.Sprintf("CREATE USER '%s'@'%s'",
			d.Get("user").(string),
			d.Get("host").(string))
	}

	var password string
	if v, ok := d.GetOk("plaintext_password"); ok {
		password = v.(string)
	} else {
		password = d.Get("password").(string)
	}

	if auth == "AWSAuthenticationPlugin" && d.Get("host").(string) == "localhost" {
		return diag.Errorf("cannot use IAM auth against localhost")
	}

	if authStm != "" {
		stmtSQL = stmtSQL + authStm
		if password != "" {
			stmtSQL = stmtSQL + fmt.Sprintf(" BY '%s'", password)
		}
	} else if password != "" {
		stmtSQL = stmtSQL + fmt.Sprintf(" IDENTIFIED BY '%s'", password)
	}

	var updateStmtSql = ""

	retainPassword := d.Get("retain_old_password").(bool)
	if retainPassword {
		err := checkRetainCurrentPasswordSupport(ctx, meta)
		if err != nil {
			return diag.Errorf("cannot use retain_current_password: %v", err)
		}
	}

	log.Println("[DEBUG] Executing statement:", stmtSQL)
	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed executing SQL: %v", err)
	}

	user := fmt.Sprintf("%s@%s", d.Get("user").(string), d.Get("host").(string))
	d.SetId(user)

	if updateStmtSql != "" {
		log.Println("[DEBUG] Executing statement:", updateStmtSql)
		_, err = db.ExecContext(ctx, updateStmtSql)
		if err != nil {
			d.Set("tls_option", "")
			return diag.Errorf("failed executing SQL: %v", err)
		}
	}

	return nil
}

func getSetPasswordStatement(ctx context.Context, meta interface{}, retainPassword bool) (string, error) {
	if retainPassword {
		return "ALTER USER ?@? IDENTIFIED BY ? RETAIN CURRENT PASSWORD", nil
	}

	return "ALTER USER ?@? IDENTIFIED BY ?", nil
}

func UpdateUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	var auth string
	if v, ok := d.GetOk("auth_plugin"); ok {
		auth = v.(string)
	}
	if len(auth) > 0 {
		if d.HasChange("tls_option") || d.HasChange("auth_plugin") || d.HasChange("auth_string_hashed") {
			var stmtSQL string

			authString := ""
			if d.Get("auth_string_hashed").(string) != "" {
				authString = fmt.Sprintf("IDENTIFIED WITH %s AS '%s'", d.Get("auth_plugin"), d.Get("auth_string_hashed"))
			}
			stmtSQL = fmt.Sprintf("ALTER USER '%s'@'%s' %s  REQUIRE %s",
				d.Get("user").(string),
				d.Get("host").(string),
				authString,
				d.Get("tls_option").(string))

			log.Println("[DEBUG] Executing query:", stmtSQL)
			_, err := db.ExecContext(ctx, stmtSQL)
			if err != nil {
				return diag.Errorf("failed running query: %v", err)
			}
		}
	}

	var newpw interface{}
	if d.HasChange("plaintext_password") {
		_, newpw = d.GetChange("plaintext_password")
	} else if d.HasChange("password") {
		_, newpw = d.GetChange("password")
	} else {
		newpw = nil
	}

	retainPassword := d.Get("retain_old_password").(bool)
	if retainPassword {
		err := checkRetainCurrentPasswordSupport(ctx, meta)
		if err != nil {
			return diag.Errorf("cannot use retain_current_password: %v", err)
		}
	}

	if newpw != nil {
		stmtSQL, err := getSetPasswordStatement(ctx, meta, retainPassword)
		if err != nil {
			return diag.Errorf("failed getting change password statement: %v", err)
		}

		log.Println("[DEBUG] Executing query:", stmtSQL)
		_, err = db.ExecContext(ctx, stmtSQL,
			d.Get("user").(string),
			d.Get("host").(string),
			newpw.(string))
		if err != nil {
			return diag.Errorf("failed changing password: %v", err)
		}
	}

	return nil
}

func ReadUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	// Worse user detection, only for compat with MySQL 5.6
	stmtSQL := fmt.Sprintf("SELECT USER FROM mysql.user WHERE USER='%s'",
		d.Get("user").(string))

	log.Println("[DEBUG] Executing statement:", stmtSQL)

	rows, err := db.QueryContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed getting user from DB: %v", err)
	}
	defer rows.Close()

	if !rows.Next() && rows.Err() == nil {
		d.SetId("")
		return nil
	}
	if rows.Err() != nil {
		return diag.Errorf("failed getting rows: %v", rows.Err())
	}
	return nil
}

func DeleteUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL := fmt.Sprintf("DROP USER ?@?")

	log.Println("[DEBUG] Executing statement:", stmtSQL)

	_, err = db.ExecContext(ctx, stmtSQL,
		d.Get("user").(string),
		d.Get("host").(string))

	if err == nil {
		d.SetId("")
	}
	return diag.FromErr(err)
}

func ImportUser(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	userHost := strings.SplitN(d.Id(), "@", 2)

	if len(userHost) != 2 {
		return nil, fmt.Errorf("wrong ID format %s (expected USER@HOST)", d.Id())
	}

	user := userHost[0]
	host := userHost[1]
	d.Set("user", user)
	d.Set("host", host)
	err := ReadUser(ctx, d, meta)
	var ferror error
	if err.HasError() {
		ferror = fmt.Errorf("failed reading user: %v", err)
	}

	return []*schema.ResourceData{d}, ferror
}

func NewEmptyStringSuppressFunc(k, old, new string, d *schema.ResourceData) bool {
	if new == "" {
		return true
	}

	return false
}
