package mysql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
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
			"auth_string_hex": {
				Type:             schema.TypeString,
				Optional:         true,
				Sensitive:        true,
				DiffSuppressFunc: NewEmptyStringSuppressFunc,
				ConflictsWith:    []string{"plaintext_password", "password", "auth_string_hashed"},
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

			"discard_old_password": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
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

func checkDiscardOldPasswordSupport(ctx context.Context, meta interface{}) error {
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

	var hashed string
	if v, ok := d.GetOk("auth_string_hashed"); ok {
		hashed = v.(string)
		if hashed != "" {
			if authStm == "" {
				return diag.Errorf("auth_string_hashed is not supported for auth plugin %s", auth)
			}
			authStm = fmt.Sprintf("%s AS ?", authStm)
		}
	}
	var hashed_hex string
	if v, ok := d.GetOk("auth_string_hex"); ok {
		hashed_hex = v.(string)
		if hashed_hex != "" {
			if hashed != "" {
				return diag.Errorf("can not specify both auth_string_hashed and auth_string_hex")
			}
			if authStm == "" {
				return diag.Errorf("auth_string_hex is not supported for auth plugin %s", auth)
			}
			if strings.HasPrefix(hashed_hex, "0x") || strings.HasPrefix(hashed_hex, "0X") {
				hashed_hex = hashed_hex[2:]
			}

			if err := validateHexString(hashed_hex); err != nil {
				return diag.Errorf("invalid hex string for auth_string_hex: %v", err)
			}
			authStm = fmt.Sprintf("%s AS 0x%s", authStm, hashed_hex)
		}

	}
	user := d.Get("user").(string)
	host := d.Get("host").(string)

	var stmtSQL string
	var args []interface{}

	if createObj == "AADUSER" {
		var aadIdentity = d.Get("aad_identity").(*schema.Set).List()[0].(map[string]interface{})
		if aadIdentity["type"].(string) == "service_principal" {
			// CREATE AADUSER 'mysqlProtocolLoginName"@"mysqlHostRestriction' IDENTIFIED BY 'identityId'
			stmtSQL = "CREATE AADUSER ?@? IDENTIFIED BY ?"
			args = []interface{}{user, host, aadIdentity["identity"].(string)}
		} else {
			// CREATE AADUSER 'identityName"@"mysqlHostRestriction' AS 'mysqlProtocolLoginName'
			stmtSQL = "CREATE AADUSER ?@? AS ?"
			args = []interface{}{aadIdentity["identity"].(string), host, user}
		}
	} else {
		stmtSQL = "CREATE USER ?@?"
		args = []interface{}{user, host}
	}

	var password string
	if v, ok := d.GetOk("plaintext_password"); ok {
		password = v.(string)
	} else {
		password = d.Get("password").(string)
	}

	if auth == "AWSAuthenticationPlugin" && host == "localhost" {
		return diag.Errorf("cannot use IAM auth against localhost")
	}

	if authStm != "" {
		stmtSQL += authStm
		if hashed != "" {
			args = append(args, hashed)
		}
		if password != "" {
			stmtSQL += " BY ?"
			args = append(args, password)
		}
	} else if password != "" {
		stmtSQL += " IDENTIFIED BY ?"
		args = append(args, password)
	}

	requiredVersion, _ := version.NewVersion("5.7.0")
	var updateStmtSql string
	var updateArgs []interface{}

	if getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) && d.Get("tls_option").(string) != "" {
		if createObj == "AADUSER" {
			updateStmtSql = "ALTER USER ?@? REQUIRE " + d.Get("tls_option").(string)
			updateArgs = []interface{}{user, host}
		} else {
			stmtSQL += " REQUIRE " + d.Get("tls_option").(string)
		}
	}

	// Redact sensitive values in args for logging
	redactedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if (password != "" && arg == password) || (hashed != "" && arg == hashed) {
			redactedArgs[i] = "<SENSITIVE>"
		} else {
			redactedArgs[i] = arg
		}
	}

	log.Println("[DEBUG] Executing statement:", stmtSQL, "args:", redactedArgs)

	_, err = db.ExecContext(ctx, stmtSQL, args...)
	if err != nil {
		return diag.Errorf("failed executing SQL: %v", err)
	}

	userId := fmt.Sprintf("%s@%s", user, host)
	d.SetId(userId)

	if updateStmtSql != "" {
		log.Println("[DEBUG] Executing statement:", updateStmtSql, "args:", updateArgs)
		_, err = db.ExecContext(ctx, updateStmtSql, updateArgs...)
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

	/* ALTER USER syntax introduced in MySQL 5.7.6 deprecates SET PASSWORD (GH-8230) */
	ver, _ := version.NewVersion("5.7.6")
	if getVersionFromMeta(ctx, meta).LessThan(ver) {
		return "SET PASSWORD FOR ?@? = PASSWORD(?)", nil
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

	discardOldPassword := d.Get("discard_old_password").(bool)
	if discardOldPassword {
		err := checkDiscardOldPasswordSupport(ctx, meta)
		if err != nil {
			return diag.Errorf("cannot use discard_old_password: %v", err)
		} else {
			var stmtSQL string
			stmtSQL = fmt.Sprintf("ALTER USER '%s'@'%s' DISCARD OLD PASSWORD",
				d.Get("user").(string),
				d.Get("host").(string))

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

	requiredVersion, _ := version.NewVersion("5.7.0")
	if d.HasChange("tls_option") && getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) {
		var stmtSQL string

		stmtSQL = fmt.Sprintf("ALTER USER '%s'@'%s' REQUIRE %s",
			d.Get("user").(string),
			d.Get("host").(string),
			d.Get("tls_option").(string))

		log.Println("[DEBUG] Executing query:", stmtSQL)
		_, err := db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed setting require tls option: %v", err)
		}
	}

	return nil
}

func ReadUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	requiredVersion, _ := version.NewVersion("5.7.0")
	if getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) {
		stmt := "SHOW CREATE USER ?@?"

		var createUserStmt string
		err := db.QueryRowContext(ctx, stmt, d.Get("user").(string), d.Get("host").(string)).Scan(&createUserStmt)
		if err != nil {
			errorNumber := mysqlErrorNumber(err)
			if errorNumber == unknownUserErrCode || errorNumber == userNotFoundErrCode {
				d.SetId("")
				return nil
			}
			return diag.Errorf("failed getting user: %v", err)
		}

		// Examples of create user:
		// CREATE USER 'some_app'@'%' IDENTIFIED WITH 'mysql_native_password' AS '*0something' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK
		// CREATE USER `jdoe-tf-test-47`@`example.com` IDENTIFIED WITH 'caching_sha2_password' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT
		// CREATE USER `jdoe`@`example.com` IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$i`xay#fG/\' TrbkNA82' REQUIRE NONE PASSWORD
		re := regexp.MustCompile("^CREATE USER ['`]([^'`]*)['`]@['`]([^'`]*)['`] IDENTIFIED WITH ['`]([^'`]*)['`] (?:AS '((?:.*?[^\\\\])?)' )?REQUIRE ([^ ]*)")
		if m := re.FindStringSubmatch(createUserStmt); len(m) == 6 {
			d.Set("user", m[1])
			d.Set("host", m[2])
			d.Set("auth_plugin", m[3])
			d.Set("tls_option", m[5])

			if m[3] == "aad_auth" {
				// AADGroup:98e61c8d-e104-4f8c-b1a6-7ae873617fe6:upn:Doe_Family_Group
				// AADUser:98e61c8d-e104-4f8c-b1a6-7ae873617fe6:upn:little.johny@does.onmicrosoft.com
				// AADSP:98e61c8d-e104-4f8c-b1a6-7ae873617fe6:upn:mysqlUserName - for MySQL Flexible Server
				// AADApp:98e61c8d-e104-4f8c-b1a6-7ae873617fe6:upn:mysqlUserName - for MySQL Single Server
				parts := strings.Split(m[4], ":")
				if parts[0] == "AADSP" || parts[0] == "AADApp" {
					// service principals are referenced by UUID only
					d.Set("aad_identity", []map[string]interface{}{
						{
							"type":     "service_principal",
							"identity": parts[1],
						},
					})
				} else if len(parts) >= 4 {
					// users and groups should be referenced by UPN / group name
					if parts[0] == "AADUser" {
						d.Set("aad_identity", []map[string]interface{}{
							{
								"type":     "user",
								"identity": strings.Join(parts[3:], ":"),
							},
						})
					} else {
						d.Set("aad_identity", []map[string]interface{}{
							{
								"type":     "group",
								"identity": strings.Join(parts[3:], ":"),
							},
						})
					}
				} else {
					return diag.Errorf("AAD identity couldn't be parsed - it is %s", m[4])
				}
			} else {
				d.Set("auth_string_hashed", m[4])
			}
			return nil
		}

		// Try 2 - just whether the user is there.
		re2 := regexp.MustCompile("^CREATE USER")
		if m := re2.FindStringSubmatch(createUserStmt); m != nil {
			// Ok, we have at least something - it's probably in MariaDB.
			return nil
		}
		return diag.Errorf("Create user couldn't be parsed - it is %s", createUserStmt)
	} else {
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

func validateHexString(hexStr string) error {
	if len(hexStr) == 0 {
		return fmt.Errorf("hex string cannot be empty")
	}

	if len(hexStr)%2 != 0 {
		return fmt.Errorf("hex string must have even length")
	}

	for i, char := range strings.ToLower(hexStr) {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return fmt.Errorf("invalid hex character '%c' at position %d", char, i)
		}
	}

	return nil
}
