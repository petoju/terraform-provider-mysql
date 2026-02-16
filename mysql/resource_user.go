package mysql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// formatUserIdentifier formats a user identifier with proper quoting for MySQL
func formatUserIdentifier(user, host string) string {
	return fmt.Sprintf("%s@%s", quoteIdentifier(user), quoteIdentifier(host))
}

// quoteString escapes and quotes a string literal for MySQL
func quoteString(s string) string {
	// MySQL string literals need to escape: backslash, single quote, double quote, null, newline, carriage return
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`"`, `\"`,
		"\x00", `\0`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return fmt.Sprintf("'%s'", replacer.Replace(s))
}

func resourceUser() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateUser,
		UpdateContext: UpdateUser,
		ReadContext:   ReadUser,
		DeleteContext: DeleteUser,
		Importer: &schema.ResourceImporter{
			StateContext: ImportUser,
		},

		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
			// Validate max_user_connections is not set on TiDB
			if _, ok := d.GetOk("max_user_connections"); ok {
				if err := checkMaxUserConnectionsSupport(ctx, meta); err != nil {
					return err
				}
			}

			// Validate max_statement_time is not set on non-MariaDB
			if _, ok := d.GetOk("max_statement_time"); ok {
				if err := checkMaxStatementTimeSupport(ctx, meta); err != nil {
					return err
				}
			}

			return nil
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
				ConflictsWith: []string{"plaintext_password", "password_wo"},
				Sensitive:     true,
				Deprecated:    "Please use plaintext_password instead",
			},

			"password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"plaintext_password", "password"},
				RequiredWith:  []string{"password_wo_version"},
				Sensitive:     true,
				WriteOnly:     true,
			},

			"password_wo_version": {
				Type:         schema.TypeInt,
				Optional:     true,
				RequiredWith: []string{"password_wo"},
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
				ConflictsWith:    []string{"plaintext_password", "password", "password_wo"},
			},
			"auth_string_hex": {
				Type:             schema.TypeString,
				Optional:         true,
				Sensitive:        true,
				StateFunc:        NormalizeHexStringStateFunc,
				DiffSuppressFunc: SuppressHexStringDiff,
				ConflictsWith:    []string{"plaintext_password", "password", "password_wo", "auth_string_hashed"},
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

			"max_user_connections": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Maximum number of simultaneous connections for the user (0 = unlimited). Supported on MySQL 5.0+ and MariaDB.",
			},

			"max_statement_time": {
				Type:         schema.TypeFloat,
				Optional:     true,
				ValidateFunc: validation.FloatAtLeast(0),
				Description:  "Maximum execution time for statements in seconds (0 = unlimited). Supports fractional values (e.g., 0.01 for 10ms, 30.5 for 30.5s). Only supported on MariaDB 10.1.1+, not MySQL.",
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

func checkMaxUserConnectionsSupport(ctx context.Context, meta interface{}) error {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return err
	}

	isTiDB, _, _, err := serverTiDB(db)
	if err != nil {
		return err
	}

	if isTiDB {
		return errors.New("MAX_USER_CONNECTIONS is not supported on TiDB")
	}

	return nil
}

func checkMaxStatementTimeSupport(ctx context.Context, meta interface{}) error {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return err
	}

	isMariaDB, err := serverMariaDB(db)
	if err != nil {
		return err
	}

	if !isMariaDB {
		return errors.New("MAX_STATEMENT_TIME is only supported on MariaDB 10.1.1+, not MySQL")
	}

	// Check MariaDB version
	currentVer := getVersionFromMeta(ctx, meta)
	minVer, _ := version.NewVersion("10.1.1")

	if currentVer.LessThan(minVer) {
		return fmt.Errorf("MAX_STATEMENT_TIME requires MariaDB 10.1.1 or newer (current version: %s)", currentVer.String())
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
	var hashedHex string
	if v, ok := d.GetOk("auth_string_hex"); ok {
		hashedHex = v.(string)
		if hashedHex != "" {
			if hashed != "" {
				return diag.Errorf("can not specify both auth_string_hashed and auth_string_hex")
			}
			if authStm == "" {
				return diag.Errorf("auth_string_hex is not supported for auth plugin %s", auth)
			}
			normalizedHex := normalizeHexString(hashedHex)
			hexDigits := normalizedHex[2:] // Remove the "0x" prefix for validation

			if err := validateHexString(hexDigits); err != nil {
				return diag.Errorf("invalid hex string for auth_string_hex: %v", err)
			}
			authStm = fmt.Sprintf("%s AS 0x%s", authStm, hexDigits)
		}

	}
	user := d.Get("user").(string)
	host := d.Get("host").(string)

	var stmtSQL string

	if createObj == "AADUSER" {
		var aadIdentity = d.Get("aad_identity").(*schema.Set).List()[0].(map[string]interface{})
		if aadIdentity["type"].(string) == "service_principal" {
			// CREATE AADUSER 'mysqlProtocolLoginName"@"mysqlHostRestriction' IDENTIFIED BY 'identityId'
			stmtSQL = fmt.Sprintf("CREATE AADUSER %s IDENTIFIED BY %s", formatUserIdentifier(user, host), quoteString(aadIdentity["identity"].(string)))
		} else {
			// CREATE AADUSER 'identityName"@"mysqlHostRestriction' AS 'mysqlProtocolLoginName'
			stmtSQL = fmt.Sprintf("CREATE AADUSER %s AS %s", formatUserIdentifier(aadIdentity["identity"].(string), host), quoteString(user))
		}
	} else {
		stmtSQL = fmt.Sprintf("CREATE USER %s", formatUserIdentifier(user, host))
	}

	var password string
	if v, ok := d.GetOk("plaintext_password"); ok {
		password = v.(string)
	} else {
		password = d.Get("password").(string)
	}

	if wo, diags := getWriteOnlyString(d, "password_wo"); diags.HasError() {
		return diags
	} else if wo != "" {
		password = wo
	}

	if auth == "AWSAuthenticationPlugin" && host == "localhost" {
		return diag.Errorf("cannot use IAM auth against localhost")
	}

	if authStm != "" {
		// Handle auth_string_hashed case
		if hashed != "" {
			// authStm already contains " AS ?" from line 197
			stmtSQL += strings.Replace(authStm, " AS ?", fmt.Sprintf(" AS %s", quoteString(hashed)), 1)
		} else {
			stmtSQL += authStm
		}
		if password != "" {
			stmtSQL += fmt.Sprintf(" BY %s", quoteString(password))
		}
	} else if password != "" {
		stmtSQL += fmt.Sprintf(" IDENTIFIED BY %s", quoteString(password))
	}

	requiredVersion, _ := version.NewVersion("5.7.0")
	var updateStmtSql string
	var updateArgs []interface{}

	if getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) && d.Get("tls_option").(string) != "" {
		if createObj == "AADUSER" {
			updateStmtSql = fmt.Sprintf("ALTER USER %s REQUIRE %s", formatUserIdentifier(user, host), d.Get("tls_option").(string))
			updateArgs = []interface{}{}
		} else {
			stmtSQL += " REQUIRE " + d.Get("tls_option").(string)
		}
	}

	// Add resource limits if specified
	// Note: MySQL 5.6 does NOT support CREATE USER ... WITH for resource limits
	// MySQL 5.7.6+ supports both CREATE USER ... WITH and ALTER USER ... WITH
	// For MySQL < 5.7.6, we need to use GRANT USAGE after CREATE USER
	var resourceLimits []string
	if createObj != "AADUSER" {
		// MAX_USER_CONNECTIONS - supported on MySQL and MariaDB, but not TiDB
		if maxConn, ok := d.GetOk("max_user_connections"); ok {
			if err := checkMaxUserConnectionsSupport(ctx, meta); err != nil {
				return diag.FromErr(err)
			}
			resourceLimits = append(resourceLimits, fmt.Sprintf("MAX_USER_CONNECTIONS %d", maxConn.(int)))
		}

		// MAX_STATEMENT_TIME - MariaDB only
		if maxStmt, ok := d.GetOk("max_statement_time"); ok {
			if err := checkMaxStatementTimeSupport(ctx, meta); err != nil {
				return diag.FromErr(err)
			}
			resourceLimits = append(resourceLimits, fmt.Sprintf("MAX_STATEMENT_TIME %f", maxStmt.(float64)))
		}

		// MySQL 5.7.6+ supports CREATE USER ... WITH for resource limits
		createUserWithVersion, _ := version.NewVersion("5.7.6")
		if len(resourceLimits) > 0 && getVersionFromMeta(ctx, meta).GreaterThanOrEqual(createUserWithVersion) {
			stmtSQL += " WITH " + strings.Join(resourceLimits, " ")
		}
	}

	// Log statement with sensitive values redacted
	logStmt := stmtSQL
	if password != "" {
		logStmt = strings.Replace(logStmt, quoteString(password), "<SENSITIVE>", -1)
	}
	if hashed != "" {
		logStmt = strings.Replace(logStmt, quoteString(hashed), "<SENSITIVE>", -1)
	}
	log.Println("[DEBUG] Executing statement:", logStmt)

	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed executing SQL: %v", err)
	}

	// For MySQL < 5.7.6, use GRANT USAGE to set resource limits after CREATE USER
	createUserWithVersion, _ := version.NewVersion("5.7.6")
	if createObj != "AADUSER" && len(resourceLimits) > 0 && getVersionFromMeta(ctx, meta).LessThan(createUserWithVersion) {
		grantStmtSQL := fmt.Sprintf("GRANT USAGE ON *.* TO %s WITH %s",
			formatUserIdentifier(user, host),
			strings.Join(resourceLimits, " "))

		log.Println("[DEBUG] Executing statement:", grantStmtSQL)
		_, err = db.ExecContext(ctx, grantStmtSQL)
		if err != nil {
			return diag.Errorf("failed setting user resource limits: %v", err)
		}
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

func getSetPasswordStatement(ctx context.Context, meta interface{}, user, host, password string, retainPassword bool) (string, error) {
	if retainPassword {
		return fmt.Sprintf("ALTER USER %s IDENTIFIED BY %s RETAIN CURRENT PASSWORD", formatUserIdentifier(user, host), quoteString(password)), nil
	}

	/* ALTER USER syntax introduced in MySQL 5.7.6 deprecates SET PASSWORD (GH-8230) */
	ver, _ := version.NewVersion("5.7.6")
	if getVersionFromMeta(ctx, meta).LessThan(ver) {
		return fmt.Sprintf("SET PASSWORD FOR %s = PASSWORD(%s)", formatUserIdentifier(user, host), quoteString(password)), nil
	}

	return fmt.Sprintf("ALTER USER %s IDENTIFIED BY %s", formatUserIdentifier(user, host), quoteString(password)), nil
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
		if d.HasChange("tls_option") || d.HasChange("auth_plugin") || d.HasChange("auth_string_hashed") || d.HasChange("auth_string_hex") {
			var stmtSQL string

			authString := ""
			if d.Get("auth_string_hashed").(string) != "" {
				authString = fmt.Sprintf("IDENTIFIED WITH %s AS '%s'", d.Get("auth_plugin"), d.Get("auth_string_hashed"))
			} else if d.Get("auth_string_hex").(string) != "" {
				authStringHex := d.Get("auth_string_hex").(string)
				normalizedHex := normalizeHexString(authStringHex)

				hexDigits := normalizedHex[2:]
				if err := validateHexString(hexDigits); err != nil {
					return diag.Errorf("invalid hex string for auth_string_hex: %v", err)
				}
				authString = fmt.Sprintf("IDENTIFIED WITH %s AS 0x%s", d.Get("auth_plugin"), hexDigits)
			}
			stmtSQL = fmt.Sprintf("ALTER USER %s %s  REQUIRE %s",
				formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)),
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
			stmtSQL = fmt.Sprintf("ALTER USER %s DISCARD OLD PASSWORD",
				formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)))

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

	if d.HasChange("password_wo_version") {
		if wo, diags := getWriteOnlyString(d, "password_wo"); diags.HasError() {
			return diags
		} else {
			newpw = wo
		}
	}

	retainPassword := d.Get("retain_old_password").(bool)
	if retainPassword {
		err := checkRetainCurrentPasswordSupport(ctx, meta)
		if err != nil {
			return diag.Errorf("cannot use retain_current_password: %v", err)
		}
	}

	if newpw != nil {
		stmtSQL, err := getSetPasswordStatement(ctx, meta, d.Get("user").(string), d.Get("host").(string), newpw.(string), retainPassword)
		if err != nil {
			return diag.Errorf("failed getting change password statement: %v", err)
		}

		// Log with password redacted
		logStmt := strings.Replace(stmtSQL, quoteString(newpw.(string)), "<SENSITIVE>", -1)
		log.Println("[DEBUG] Executing query:", logStmt)
		_, err = db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed changing password: %v", err)
		}
	}

	requiredVersion, _ := version.NewVersion("5.7.0")
	if d.HasChange("tls_option") && getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) {
		var stmtSQL string

		stmtSQL = fmt.Sprintf("ALTER USER %s REQUIRE %s",
			formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)),
			d.Get("tls_option").(string))

		log.Println("[DEBUG] Executing query:", stmtSQL)
		_, err := db.ExecContext(ctx, stmtSQL)
		if err != nil {
			return diag.Errorf("failed setting require tls option: %v", err)
		}
	}

	// Handle resource limits changes (Option B: field removal resets to 0)
	// MySQL 5.6: ALTER USER only supports PASSWORD EXPIRE, use GRANT USAGE for resource limits
	// MySQL 5.7.6+: ALTER USER supports WITH clause for resource limits
	if d.HasChange("max_user_connections") || d.HasChange("max_statement_time") {
		var resourceLimits []string

		// Handle MAX_USER_CONNECTIONS
		if maxConn, ok := d.GetOk("max_user_connections"); ok {
			// Field is present in config, validate and set the value
			if err := checkMaxUserConnectionsSupport(ctx, meta); err != nil {
				return diag.FromErr(err)
			}
			resourceLimits = append(resourceLimits, fmt.Sprintf("MAX_USER_CONNECTIONS %d", maxConn.(int)))
		} else if d.HasChange("max_user_connections") {
			// Field was removed from config, reset to 0 (unlimited)
			// Only reset if we're not on TiDB (which doesn't support this feature)
			isTiDBVal, _, _, err := serverTiDB(db)
			if err != nil {
				return diag.FromErr(err)
			}
			if !isTiDBVal {
				resourceLimits = append(resourceLimits, "MAX_USER_CONNECTIONS 0")
			} else {
				return diag.Errorf("cannot reset max_user_connections on TiDB: MAX_USER_CONNECTIONS is not supported on TiDB")
			}
		}

		// Handle MAX_STATEMENT_TIME (MariaDB only)
		if maxStmt, ok := d.GetOk("max_statement_time"); ok {
			// Field is present in config, validate and set the value
			if err := checkMaxStatementTimeSupport(ctx, meta); err != nil {
				return diag.FromErr(err)
			}
			resourceLimits = append(resourceLimits, fmt.Sprintf("MAX_STATEMENT_TIME %f", maxStmt.(float64)))
		} else if d.HasChange("max_statement_time") {
			// Field was removed from config, reset to 0 (unlimited)
			// Only reset if we're on MariaDB (no need to check version, just database type)
			isMariaDBVal, err := serverMariaDB(db)
			if err != nil {
				return diag.FromErr(err)
			}
			if isMariaDBVal {
				resourceLimits = append(resourceLimits, "MAX_STATEMENT_TIME 0")
			}
		}

		if len(resourceLimits) > 0 {
			var stmtSQL string

			// MySQL versions before 5.7.6 don't support ALTER USER with WITH clause
			// Use GRANT USAGE instead for older versions
			alterUserVersion, _ := version.NewVersion("5.7.6")
			if getVersionFromMeta(ctx, meta).LessThan(alterUserVersion) {
				// MySQL 5.6 and earlier: use GRANT USAGE
				stmtSQL = fmt.Sprintf("GRANT USAGE ON *.* TO %s WITH %s",
					formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)),
					strings.Join(resourceLimits, " "))
			} else {
				// MySQL 5.7.6+: use ALTER USER
				stmtSQL = fmt.Sprintf("ALTER USER %s WITH %s",
					formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)),
					strings.Join(resourceLimits, " "))
			}

			log.Println("[DEBUG] Executing query:", stmtSQL)
			_, err := db.ExecContext(ctx, stmtSQL)
			if err != nil {
				return diag.Errorf("failed setting user resource limits: %v", err)
			}
		}
	}

	return nil
}

// parseWithClauseSetting extracts and sets a resource limit from the WITH clause
func parseWithClauseSetting(d *schema.ResourceData, withClause, fieldName, settingName string, parseAsFloat bool) {
	// Only set if the field is currently being managed (Option B behavior)
	if _, ok := d.GetOk(fieldName); !ok {
		return
	}

	pattern := fmt.Sprintf(`%s\s+([\d.]+)`, settingName)
	re := regexp.MustCompile(pattern)

	if match := re.FindStringSubmatch(withClause); len(match) > 1 {
		if parseAsFloat {
			if value, err := strconv.ParseFloat(match[1], 64); err == nil {
				d.Set(fieldName, value)
			}
		} else {
			if value, err := strconv.Atoi(match[1]); err == nil {
				d.Set(fieldName, value)
			}
		}
	}
}

func ReadUser(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	requiredVersion, _ := version.NewVersion("5.7.0")
	if getVersionFromMeta(ctx, meta).GreaterThan(requiredVersion) {
		// Skip setting print_identified_with_as_hex if auth_plugin is aad_auth
		if d.Get("auth_plugin") != "aad_auth" {
			_, err := db.ExecContext(ctx, "SET print_identified_with_as_hex = ON")
			if err != nil {
				// return diag.Errorf("failed setting print_identified_with_as_hex: %v", err)
				log.Printf("[DEBUG] Could not set print_identified_with_as_hex: %v", err)
			}
		}
		stmt := fmt.Sprintf("SHOW CREATE USER %s", formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)))
		var createUserStmt string
		err = db.QueryRowContext(ctx, stmt).Scan(&createUserStmt)
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
		// CREATE USER `hashed_hex`@`localhost` IDENTIFIED WITH 'caching_sha2_password' AS 0x244124303035242522434C16580334755221766C29210D2C415E033550367655494F314864686775414E735A742E6F474857504B623172525066574D524F30506B7A79646F30 REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK PASSWORD HISTORY DEFAULT PASSWORD REUSE INTERVAL DEFAULT PASSWORD REQUIRE CURRENT DEFAULT

		re := regexp.MustCompile("^CREATE USER ['`]([^'`]*)['`]@['`]([^'`]*)['`] IDENTIFIED WITH ['`]([^'`]*)['`] (?:AS (?:'((?:.*?[^\\\\])?)'|(0x[0-9A-Fa-f]+)) )?REQUIRE ([^ ]*)")
		if m := re.FindStringSubmatch(createUserStmt); len(m) == 7 {
			d.Set("user", m[1])
			d.Set("host", m[2])
			d.Set("auth_plugin", m[3])
			d.Set("tls_option", m[6])

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
				quotedAuthString := m[4]
				authStringHex := m[5]

				if authStringHex != "" {
					normalizedHex := normalizeHexString(authStringHex)
					d.Set("auth_string_hex", normalizedHex)
					d.Set("auth_string_hashed", "")
				} else if quotedAuthString != "" {
					d.Set("auth_string_hashed", quotedAuthString)
					d.Set("auth_string_hex", "")
				} else {
					d.Set("auth_string_hashed", "")
					d.Set("auth_string_hex", "")
				}
			}

			// Parse resource limits from WITH clause if present
			// Examples of WITH clause in CREATE USER:
			// CREATE USER 'user'@'host' ... WITH MAX_USER_CONNECTIONS 10
			// CREATE USER 'user'@'host' ... WITH MAX_STATEMENT_TIME 30.5 (MariaDB only)
			// CREATE USER 'user'@'host' ... WITH MAX_USER_CONNECTIONS 5 MAX_STATEMENT_TIME 60.0
			withRe := regexp.MustCompile(`WITH\s+(.*)$`)
			if withMatch := withRe.FindStringSubmatch(createUserStmt); len(withMatch) > 1 {
				withClause := withMatch[1]

				parseWithClauseSetting(d, withClause, "max_user_connections", "MAX_USER_CONNECTIONS", false)
				parseWithClauseSetting(d, withClause, "max_statement_time", "MAX_STATEMENT_TIME", true)
			}

			return nil
		}

		// Try 2 - just whether the user is there.
		re2 := regexp.MustCompile("^CREATE USER")
		if m := re2.FindStringSubmatch(createUserStmt); m != nil {
			// Ok, we have at least something - it's probably in MariaDB.
			// Parse resource limits from WITH clause if present (MariaDB format)
			withRe := regexp.MustCompile(`WITH\s+(.*)$`)
			if withMatch := withRe.FindStringSubmatch(createUserStmt); len(withMatch) > 1 {
				withClause := withMatch[1]

				parseWithClauseSetting(d, withClause, "max_user_connections", "MAX_USER_CONNECTIONS", false)
				parseWithClauseSetting(d, withClause, "max_statement_time", "MAX_STATEMENT_TIME", true)
			}

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

	stmtSQL := fmt.Sprintf("DROP USER %s", formatUserIdentifier(d.Get("user").(string), d.Get("host").(string)))

	log.Println("[DEBUG] Executing statement:", stmtSQL)

	_, err = db.ExecContext(ctx, stmtSQL)

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

func getWriteOnlyString(d *schema.ResourceData, pathName string) (string, diag.Diagnostics) {
	path := cty.GetAttrPath(pathName)
	if d.GetRawConfig().IsNull() {
		return "", diag.Diagnostics{}
	}
	val, di := d.GetRawConfigAt(path)
	if di.HasError() {
		return "", di
	}
	if !val.Type().Equals(cty.String) {
		return "", diag.Errorf("invalid type for %s, expected string", pathName)
	}
	if val.IsNull() || !val.IsKnown() {
		return "", diag.Diagnostics{}
	}
	value := val.AsString()
	if value == "" {
		return "", diag.Errorf("%s must not be empty", pathName)
	}
	return value, nil
}

func NewEmptyStringSuppressFunc(k, old, new string, d *schema.ResourceData) bool {
	if new == "" {
		return true
	}

	return false
}
func SuppressHexStringDiff(k, old, new string, d *schema.ResourceData) bool {
	if new == "" {
		return true
	}

	// Normalize both values and compare
	normalizedOld := normalizeHexString(old)
	normalizedNew := normalizeHexString(new)

	// Suppress diff if they're the same after normalization
	if normalizedOld == normalizedNew {
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

func NormalizeHexStringStateFunc(val interface{}) string {
	if val == nil {
		return ""
	}

	hexStr := val.(string)
	return normalizeHexString(hexStr) // Always store normalized format
}

// Add this helper function to normalize hex strings
func normalizeHexString(hexStr string) string {
	if hexStr == "" {
		return ""
	}

	// Remove 0x prefix if present
	if strings.HasPrefix(hexStr, "0x") || strings.HasPrefix(hexStr, "0X") {
		hexStr = hexStr[2:]
	}

	// Convert to lowercase for consistency
	hexStr = strings.ToUpper(hexStr)

	// Always return with 0x prefix for consistency
	return "0x" + hexStr
}
