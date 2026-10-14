package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const unknownProcedureErrCode = 1305

// procedureIDSeparator separates the database from the procedure name in the
// resource ID, mirroring how MySQL itself qualifies a routine: `db`.`proc`.
const procedureIDSeparator = "."

var procedureParameterModes = []string{"IN", "OUT", "INOUT"}

var procedureSQLDataAccess = []string{
	"CONTAINS SQL",
	"NO SQL",
	"READS SQL DATA",
	"MODIFIES SQL DATA",
}

var procedureSecurityTypes = []string{"DEFINER", "INVOKER"}

// procedureCharacteristics lists the keyword sequences of the characteristics
// that may precede a procedure body, except COMMENT which takes a value.
var procedureCharacteristics = [][]string{
	{"LANGUAGE", "SQL"},
	{"NOT", "DETERMINISTIC"},
	{"DETERMINISTIC"},
	{"CONTAINS", "SQL"},
	{"NO", "SQL"},
	{"READS", "SQL", "DATA"},
	{"MODIFIES", "SQL", "DATA"},
	{"SQL", "SECURITY", "DEFINER"},
	{"SQL", "SECURITY", "INVOKER"},
}

const whitespace = " \t\r\n"

func resourceProcedure() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateProcedure,
		ReadContext:   ReadProcedure,
		UpdateContext: UpdateProcedure,
		DeleteContext: DeleteProcedure,
		Importer: &schema.ResourceImporter{
			StateContext: ImportProcedure,
		},

		Schema: map[string]*schema.Schema{
			"database": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"definer": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				StateFunc:    normalizeDefiner,
				ValidateFunc: validateDefiner,
			},

			"parameter": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"mode": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      "IN",
							StateFunc:    upperCase,
							ValidateFunc: validation.StringInSlice(procedureParameterModes, true),
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},

			"body": {
				Type:      schema.TypeString,
				Required:  true,
				StateFunc: trimBody,
			},

			"comment": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},

			"deterministic": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"sql_data_access": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "CONTAINS SQL",
				StateFunc:    upperCase,
				ValidateFunc: validation.StringInSlice(procedureSQLDataAccess, true),
			},

			"security_type": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "DEFINER",
				StateFunc:    upperCase,
				ValidateFunc: validation.StringInSlice(procedureSecurityTypes, true),
			},
		},
	}
}

func CreateProcedure(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL, err := procedureCreateSQL(d.Get, false)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Println("[DEBUG] Executing statement:", stmtSQL)

	// CREATE PROCEDURE cannot be prepared, so the statement is passed without
	// any placeholder arguments to keep the driver from server-side preparing it.
	if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
		return diag.Errorf("failed creating procedure: %v", err)
	}

	d.SetId(procedureID(d.Get("database").(string), d.Get("name").(string)))

	return ReadProcedure(ctx, d, meta)
}

func ReadProcedure(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := readProcedure(ctx, db, d); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func UpdateProcedure(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	database, name, err := parseProcedureID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	// ALTER PROCEDURE cannot change these attributes, so the routine is
	// recreated. The CREATE statement also sets the new characteristics.
	if d.HasChanges("body", "parameter", "definer", "deterministic") {
		if err := recreateProcedure(ctx, db, d, database, name); err != nil {
			return diag.FromErr(err)
		}
		return ReadProcedure(ctx, d, meta)
	}

	var characteristics []string
	if d.HasChange("comment") {
		characteristics = append(characteristics, "COMMENT "+quoteString(d.Get("comment").(string)))
	}
	if d.HasChange("sql_data_access") {
		characteristics = append(characteristics, strings.ToUpper(d.Get("sql_data_access").(string)))
	}
	if d.HasChange("security_type") {
		characteristics = append(characteristics, "SQL SECURITY "+strings.ToUpper(d.Get("security_type").(string)))
	}

	if len(characteristics) > 0 {
		stmtSQL := fmt.Sprintf("ALTER PROCEDURE %s %s", qualifiedProcedureName(database, name), strings.Join(characteristics, " "))
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
			return diag.Errorf("failed updating procedure: %v", err)
		}
	}

	return ReadProcedure(ctx, d, meta)
}

// recreateProcedure replaces the procedure with its new definition and keeps
// the privileges that accounts hold on it.
//
// MariaDB does this in one statement: CREATE OR REPLACE PROCEDURE keeps the
// privileges, and a failed statement leaves the old procedure in place.
//
// MySQL has no CREATE OR REPLACE PROCEDURE. With automatic_sp_privileges ON,
// the default, DROP PROCEDURE revokes the privileges of every account on the
// routine. So the privileges are read from mysql.procs_priv first and granted
// again after the CREATE. If the CREATE fails, the old definition and its
// privileges are restored.
func recreateProcedure(ctx context.Context, db *sql.DB, d *schema.ResourceData, database, name string) error {
	newValue, oldValue := d.Get, oldValue(d)

	// Without a configured definer, CREATE makes the connecting account the
	// definer, as on the first create. Naming the definer from state instead
	// needs SET_USER_ID when it is another account, for example after an
	// import, and the restore below would then fail too.
	if config := d.GetRawConfig(); config.IsNull() || config.GetAttr("definer").IsNull() {
		newValue, oldValue = withoutDefiner(newValue), withoutDefiner(oldValue)
	}

	isMariaDB, err := serverMariaDB(db)
	if err != nil {
		return fmt.Errorf("failed detecting the server type: %w", err)
	}

	if isMariaDB {
		stmtSQL, err := procedureCreateSQL(newValue, true)
		if err != nil {
			return err
		}

		log.Println("[DEBUG] Executing statement:", stmtSQL)
		if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
			// The old procedure is unchanged, so the state must keep it too.
			d.Partial(true)
			return fmt.Errorf("failed recreating procedure: %w", err)
		}
		return nil
	}

	createSQL, err := procedureCreateSQL(newValue, false)
	if err != nil {
		return err
	}

	restoreSQL, err := procedureCreateSQL(oldValue, false)
	if err != nil {
		return err
	}

	grants, err := readProcedureGrants(ctx, db, database, name)
	if err != nil {
		return fmt.Errorf("failed reading the grants on procedure %s.%s, which a recreation drops; "+
			"the provider user needs SELECT on mysql.procs_priv: %w", database, name, err)
	}

	dropSQL := "DROP PROCEDURE " + qualifiedProcedureName(database, name)
	log.Println("[DEBUG] Executing statement:", dropSQL)
	if _, err := db.ExecContext(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed dropping procedure: %w", err)
	}

	log.Println("[DEBUG] Executing statement:", createSQL)
	if _, err := db.ExecContext(ctx, createSQL); err != nil {
		createErr := fmt.Errorf("failed recreating procedure: %w", err)
		d.Partial(true)

		log.Println("[DEBUG] Executing statement:", restoreSQL)
		if _, err := db.ExecContext(ctx, restoreSQL); err != nil {
			return fmt.Errorf("%w; restoring the previous definition also failed, so the procedure is dropped: %v", createErr, err)
		}
		if err := grantProcedurePrivileges(ctx, db, grants); err != nil {
			return fmt.Errorf("%w; the previous definition was restored, but %v", createErr, err)
		}
		return fmt.Errorf("%w; the previous definition and its grants were restored", createErr)
	}

	return grantProcedurePrivileges(ctx, db, grants)
}

// oldValue returns a getter for the values in state before the update.
func oldValue(d *schema.ResourceData) func(string) interface{} {
	return func(key string) interface{} {
		old, _ := d.GetChange(key)
		return old
	}
}

// withoutDefiner returns get, but with an empty definer, so that CREATE leaves
// out the DEFINER clause.
func withoutDefiner(get func(string) interface{}) func(string) interface{} {
	return func(key string) interface{} {
		if key == "definer" {
			return ""
		}
		return get(key)
	}
}

// readProcedureGrants returns the privileges that accounts hold on the
// procedure. Each grant keeps the database and routine name of its row,
// because a GRANT stores the routine name with the case it was given.
func readProcedureGrants(ctx context.Context, db *sql.DB, database, name string) ([]procedureGrant, error) {
	stmtSQL := "SELECT Db, Routine_name, User, Host, Proc_priv FROM mysql.procs_priv " +
		"WHERE Db = ? AND Routine_name = ? AND Routine_type = 'PROCEDURE'"
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	rows, err := db.QueryContext(ctx, stmtSQL, database, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []procedureGrant
	for rows.Next() {
		var rowDatabase, routine, user, host, procPriv string
		if err := rows.Scan(&rowDatabase, &routine, &user, &host, &procPriv); err != nil {
			return nil, err
		}

		grant, err := newProcedureGrant(rowDatabase, routine, user, host, procPriv)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}

	return grants, rows.Err()
}

// grantProcedurePrivileges grants the privileges again. It tries every grant,
// so one failure does not leave the other accounts without access.
func grantProcedurePrivileges(ctx context.Context, db *sql.DB, grants []procedureGrant) error {
	var errs []error
	for _, grant := range grants {
		stmtSQL := grant.grantSQL()
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", stmtSQL, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed restoring the grants on the procedure: %w", errors.Join(errs...))
	}

	return nil
}

func DeleteProcedure(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	database, name, err := parseProcedureID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL := "DROP PROCEDURE IF EXISTS " + qualifiedProcedureName(database, name)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
		return diag.Errorf("failed deleting procedure: %v", err)
	}

	d.SetId("")

	return nil
}

func ImportProcedure(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, err
	}

	if err := readProcedure(ctx, db, d); err != nil {
		return nil, err
	}

	if d.Id() == "" {
		return nil, fmt.Errorf("procedure %s not found", d.Id())
	}

	return []*schema.ResourceData{d}, nil
}

func readProcedure(ctx context.Context, db *sql.DB, d *schema.ResourceData) error {
	database, name, err := parseProcedureID(d.Id())
	if err != nil {
		return err
	}

	stmtSQL := "SELECT ROUTINE_COMMENT, IS_DETERMINISTIC, SQL_DATA_ACCESS, SECURITY_TYPE, DEFINER " +
		"FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'"
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	var comment, definer sql.NullString
	var isDeterministic, sqlDataAccess, securityType string
	err = db.QueryRowContext(ctx, stmtSQL, database, name).Scan(
		&comment, &isDeterministic, &sqlDataAccess, &securityType, &definer,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed reading procedure: %w", err)
	}

	parameters, body, err := readProcedureDefinition(ctx, db, database, name)
	if err != nil {
		if mysqlErrorNumber(err) == unknownProcedureErrCode {
			d.SetId("")
			return nil
		}
		return err
	}

	d.Set("database", database)
	d.Set("name", name)
	d.Set("body", body)
	d.Set("comment", comment.String)
	d.Set("deterministic", isDeterministic == "YES")
	d.Set("sql_data_access", sqlDataAccess)
	d.Set("security_type", securityType)
	d.Set("definer", normalizeDefiner(definer.String))
	d.Set("parameter", parameters)

	return nil
}

// readProcedureDefinition recovers the parameter list and the body from SHOW
// CREATE PROCEDURE, which returns both exactly as they were declared.
// information_schema cannot be used for either: PARAMETERS reports the type as
// MySQL normalized it (INT becomes int(11) on 5.7 and MariaDB but int on 8.0),
// and ROUTINES.ROUTINE_DEFINITION unescapes quotes inside string literals on
// MySQL, which is not valid SQL and would produce a permanent diff.
func readProcedureDefinition(ctx context.Context, db *sql.DB, database, name string) ([]interface{}, string, error) {
	stmtSQL := "SHOW CREATE PROCEDURE " + qualifiedProcedureName(database, name)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	createStmt, err := showCreateProcedure(ctx, db, stmtSQL)
	if err != nil {
		return nil, "", err
	}

	parameterList, remainder, err := extractProcedureParameterList(createStmt)
	if err != nil {
		return nil, "", fmt.Errorf("failed parsing %q: %w", createStmt, err)
	}

	parameters, err := parseProcedureParameterList(parameterList)
	if err != nil {
		return nil, "", err
	}

	body, err := extractProcedureBody(remainder)
	if err != nil {
		return nil, "", fmt.Errorf("failed parsing %q: %w", createStmt, err)
	}

	return parameters, body, nil
}

// showCreateProcedure runs SHOW CREATE PROCEDURE and returns the "Create
// Procedure" column. The column is looked up by name because the number of
// columns differs between server flavours and versions.
func showCreateProcedure(ctx context.Context, db *sql.DB, stmtSQL string) (string, error) {
	rows, err := db.QueryContext(ctx, stmtSQL)
	if err != nil {
		return "", fmt.Errorf("failed reading procedure definition: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed reading procedure definition columns: %w", err)
	}

	if !rows.Next() {
		if rows.Err() != nil {
			return "", fmt.Errorf("failed reading procedure definition: %w", rows.Err())
		}
		return "", fmt.Errorf("procedure definition not found")
	}

	values := make([]sql.NullString, len(columns))
	targets := make([]interface{}, len(columns))
	for i := range values {
		targets[i] = &values[i]
	}

	if err := rows.Scan(targets...); err != nil {
		return "", fmt.Errorf("failed scanning procedure definition: %w", err)
	}

	for i, column := range columns {
		if strings.EqualFold(column, "Create Procedure") {
			if !values[i].Valid {
				return "", fmt.Errorf("procedure definition is not visible, the current user may lack the required privileges")
			}
			return values[i].String, nil
		}
	}

	return "", fmt.Errorf("no %q column in SHOW CREATE PROCEDURE output", "Create Procedure")
}

// procedureCreateSQL builds the CREATE PROCEDURE statement from the values that
// get returns: the planned ones from ResourceData.Get, or the old ones when the
// previous definition is restored. orReplace is for MariaDB only.
func procedureCreateSQL(get func(string) interface{}, orReplace bool) (string, error) {
	parameters, err := procedureParametersSQL(get("parameter").([]interface{}))
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("CREATE ")
	if orReplace {
		builder.WriteString("OR REPLACE ")
	}

	if definer := get("definer").(string); definer != "" {
		user, host, err := splitDefiner(definer)
		if err != nil {
			return "", err
		}
		builder.WriteString(fmt.Sprintf("DEFINER = %s ", formatUserIdentifier(user, host)))
	}

	builder.WriteString(fmt.Sprintf(
		"PROCEDURE %s (%s)",
		qualifiedProcedureName(get("database").(string), get("name").(string)),
		parameters,
	))

	if comment := get("comment").(string); comment != "" {
		builder.WriteString("\nCOMMENT " + quoteString(comment))
	}

	if get("deterministic").(bool) {
		builder.WriteString("\nDETERMINISTIC")
	} else {
		builder.WriteString("\nNOT DETERMINISTIC")
	}

	builder.WriteString("\n" + strings.ToUpper(get("sql_data_access").(string)))
	builder.WriteString("\nSQL SECURITY " + strings.ToUpper(get("security_type").(string)))
	builder.WriteString("\n" + trimBody(get("body")))

	return builder.String(), nil
}

func procedureParametersSQL(parameters []interface{}) (string, error) {
	declarations := make([]string, 0, len(parameters))

	for i, raw := range parameters {
		parameter, ok := raw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("parameter %d is malformed", i)
		}

		parameterType := strings.TrimSpace(parameter["type"].(string))
		if parameterType == "" {
			return "", fmt.Errorf("parameter %d has an empty type", i)
		}

		declarations = append(declarations, fmt.Sprintf(
			"%s %s %s",
			strings.ToUpper(parameter["mode"].(string)),
			quoteIdentifier(parameter["name"].(string)),
			parameterType,
		))
	}

	return strings.Join(declarations, ", "), nil
}

// extractProcedureParameterList returns the text between the parentheses that
// follow the routine name in a CREATE PROCEDURE statement, along with the rest
// of the statement after the closing parenthesis.
func extractProcedureParameterList(createStmt string) (string, string, error) {
	start := -1
	depth := 0

	for i := 0; i < len(createStmt); i++ {
		switch c := createStmt[i]; c {
		case '`', '\'', '"':
			end, err := skipQuoted(createStmt, i)
			if err != nil {
				return "", "", err
			}
			i = end

		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++

		case ')':
			depth--
			if depth < 0 {
				return "", "", fmt.Errorf("unbalanced parentheses")
			}
			if depth == 0 {
				return createStmt[start:i], createStmt[i+1:], nil
			}
		}
	}

	return "", "", fmt.Errorf("no parameter list found")
}

// extractProcedureBody returns the routine body from the part of a CREATE
// PROCEDURE statement that follows the parameter list, skipping the
// characteristics that precede the body.
func extractProcedureBody(remainder string) (string, error) {
	for {
		remainder = strings.TrimLeft(remainder, whitespace)

		n, err := procedureCharacteristicLength(remainder)
		if err != nil {
			return "", err
		}
		if n == 0 {
			break
		}
		remainder = remainder[n:]
	}

	body := strings.TrimSpace(remainder)
	if body == "" {
		return "", fmt.Errorf("no procedure body found")
	}

	return body, nil
}

// procedureCharacteristicLength returns the length of the characteristic s
// starts with, or 0 when s does not start with one. No statement starts with
// a characteristic keyword, so the first mismatch marks the start of the body.
// COMMENT only counts when a string follows it: a label may be named comment.
func procedureCharacteristicLength(s string) (int, error) {
	if n := keywordsLength(s, "COMMENT"); n > 0 {
		value := strings.TrimLeft(s[n:], whitespace)
		if value != "" && (value[0] == '\'' || value[0] == '"') {
			end, err := skipQuoted(value, 0)
			if err != nil {
				return 0, err
			}
			return len(s) - len(value) + end + 1, nil
		}
	}

	for _, characteristic := range procedureCharacteristics {
		if n := keywordsLength(s, characteristic...); n > 0 {
			return n, nil
		}
	}

	return 0, nil
}

// keywordsLength returns the length of the whitespace separated keywords s
// starts with, matched case-insensitively as whole words, or 0 on a mismatch.
func keywordsLength(s string, keywords ...string) int {
	n := 0

	for i, keyword := range keywords {
		if i > 0 {
			trimmed := strings.TrimLeft(s[n:], whitespace)
			if len(trimmed) == len(s[n:]) {
				return 0
			}
			n = len(s) - len(trimmed)
		}

		rest := s[n:]
		if len(rest) < len(keyword) || !strings.EqualFold(rest[:len(keyword)], keyword) {
			return 0
		}
		n += len(keyword)

		if n < len(s) && !strings.ContainsRune(whitespace, rune(s[n])) {
			return 0
		}
	}

	return n
}

// parseProcedureParameterList splits a parameter list on its top level commas
// and parses every declaration into the resource's parameter schema.
func parseProcedureParameterList(parameterList string) ([]interface{}, error) {
	if strings.TrimSpace(parameterList) == "" {
		return []interface{}{}, nil
	}

	parameters := []interface{}{}
	depth := 0
	start := 0

	appendParameter := func(declaration string) error {
		parameter, err := parseProcedureParameter(declaration)
		if err != nil {
			return err
		}
		parameters = append(parameters, parameter)
		return nil
	}

	for i := 0; i < len(parameterList); i++ {
		switch c := parameterList[i]; c {
		case '`', '\'', '"':
			end, err := skipQuoted(parameterList, i)
			if err != nil {
				return nil, err
			}
			i = end

		case '(':
			depth++

		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parentheses in parameter list")
			}

		case ',':
			if depth == 0 {
				if err := appendParameter(parameterList[start:i]); err != nil {
					return nil, err
				}
				start = i + 1
			}
		}
	}

	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses in parameter list")
	}

	if err := appendParameter(parameterList[start:]); err != nil {
		return nil, err
	}

	return parameters, nil
}

// parseProcedureParameter parses a single declaration such as
// "INOUT `amount` DECIMAL(10,2)" into the resource's parameter schema. The
// mode is optional and defaults to IN, just like in MySQL.
func parseProcedureParameter(declaration string) (map[string]interface{}, error) {
	first, rest, err := readIdentifier(declaration)
	if err != nil {
		return nil, err
	}

	mode := "IN"
	name := first
	parameterType := strings.TrimSpace(rest)

	// A leading IN/OUT/INOUT is only a mode when a name and a type follow it;
	// a parameter may legitimately be named "in".
	if isProcedureParameterMode(first) {
		if candidate, remainder, err := readIdentifier(rest); err == nil && strings.TrimSpace(remainder) != "" {
			mode = strings.ToUpper(first)
			name = candidate
			parameterType = strings.TrimSpace(remainder)
		}
	}

	if name == "" || parameterType == "" {
		return nil, fmt.Errorf("cannot parse parameter %q", strings.TrimSpace(declaration))
	}

	return map[string]interface{}{
		"mode": mode,
		"name": name,
		"type": parameterType,
	}, nil
}

// readIdentifier reads the leading identifier of s, unquoting it when it is
// backtick quoted, and returns it along with the remainder of s.
func readIdentifier(s string) (string, string, error) {
	s = strings.TrimLeft(s, " \t\r\n")
	if s == "" {
		return "", "", fmt.Errorf("expected an identifier")
	}

	if s[0] == '`' {
		end, err := skipQuoted(s, 0)
		if err != nil {
			return "", "", err
		}
		return strings.ReplaceAll(s[1:end], "``", "`"), s[end+1:], nil
	}

	if end := strings.IndexAny(s, " \t\r\n"); end >= 0 {
		return s[:end], s[end:], nil
	}

	return s, "", nil
}

// skipQuoted returns the index of the quote closing the quoted section that
// starts at s[start]. Doubled quotes and backslash escapes are skipped over.
func skipQuoted(s string, start int) (int, error) {
	quote := s[start]

	for i := start + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if quote != '`' {
				i++
			}

		case quote:
			if i+1 < len(s) && s[i+1] == quote {
				i++
				continue
			}
			return i, nil
		}
	}

	return 0, fmt.Errorf("unterminated %c quoted section", quote)
}

func isProcedureParameterMode(token string) bool {
	for _, mode := range procedureParameterModes {
		if strings.EqualFold(token, mode) {
			return true
		}
	}
	return false
}

// splitDefiner splits a "user@host" definer, tolerating quoted components. The
// host cannot contain @, so the last one separates the two parts.
func splitDefiner(definer string) (string, string, error) {
	at := strings.LastIndex(definer, "@")
	if at < 0 {
		return "", "", fmt.Errorf("wrong definer format %q - expected user@host", definer)
	}

	user := unquote(definer[:at])
	host := unquote(definer[at+1:])
	if user == "" || host == "" {
		return "", "", fmt.Errorf("wrong definer format %q - expected user@host", definer)
	}

	return user, host, nil
}

func validateDefiner(v interface{}, k string) ([]string, []error) {
	if _, _, err := splitDefiner(v.(string)); err != nil {
		return nil, []error{fmt.Errorf("%s: %w", k, err)}
	}
	return nil, nil
}

// normalizeDefiner strips the optional quoting around both definer parts so
// that the configured value matches what the server reports back. A StateFunc
// cannot fail; malformed input is rejected by validateDefiner at plan time, so
// the fallback only passes through the empty value of an unset definer.
func normalizeDefiner(definer interface{}) string {
	user, host, err := splitDefiner(definer.(string))
	if err != nil {
		return definer.(string)
	}
	return fmt.Sprintf("%s@%s", user, host)
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}

	switch quote := s[0]; quote {
	case '`', '\'', '"':
		if s[len(s)-1] == quote {
			return strings.ReplaceAll(s[1:len(s)-1], string([]byte{quote, quote}), string(quote))
		}
	}

	return s
}

// procedurePrivileges maps the values of mysql.procs_priv.Proc_priv to the
// privilege names of GRANT. The remaining value, Grant, is the grant option.
var procedurePrivileges = map[string]string{
	"Execute":       "EXECUTE",
	"Alter Routine": "ALTER ROUTINE",
}

// procedureGrant holds the privileges of one account on one procedure, as
// stored in a row of mysql.procs_priv.
type procedureGrant struct {
	Database    string
	Routine     string
	User        string
	Host        string
	Privileges  []string
	GrantOption bool
}

func newProcedureGrant(database, routine, user, host, procPriv string) (procedureGrant, error) {
	grant := procedureGrant{Database: database, Routine: routine, User: user, Host: host}

	for _, value := range strings.Split(procPriv, ",") {
		if value == "" {
			continue
		}
		if value == "Grant" {
			grant.GrantOption = true
			continue
		}

		privilege, ok := procedurePrivileges[value]
		if !ok {
			return grant, fmt.Errorf("unknown privilege %q on procedure %s.%s", value, database, routine)
		}
		grant.Privileges = append(grant.Privileges, privilege)
	}

	return grant, nil
}

// grantSQL returns the GRANT statement that restores the privileges. A row
// that holds only the grant option was created with GRANT USAGE.
func (g procedureGrant) grantSQL() string {
	privileges := "USAGE"
	if len(g.Privileges) > 0 {
		privileges = strings.Join(g.Privileges, ", ")
	}

	stmtSQL := fmt.Sprintf("GRANT %s ON PROCEDURE %s TO %s",
		privileges, qualifiedProcedureName(g.Database, g.Routine), formatUserIdentifier(g.User, g.Host))
	if g.GrantOption {
		stmtSQL += " WITH GRANT OPTION"
	}

	return stmtSQL
}

func qualifiedProcedureName(database, name string) string {
	return fmt.Sprintf("%s.%s", quoteIdentifier(database), quoteIdentifier(name))
}

func procedureID(database, name string) string {
	return database + procedureIDSeparator + name
}

func parseProcedureID(id string) (string, string, error) {
	database, name, found := strings.Cut(id, procedureIDSeparator)
	if !found || database == "" || name == "" {
		return "", "", fmt.Errorf("wrong ID format %q - expected database%sprocedure", id, procedureIDSeparator)
	}
	return database, name, nil
}

func upperCase(v interface{}) string {
	return strings.ToUpper(v.(string))
}

// trimBody strips the surrounding whitespace and any trailing statement
// terminator. The server stores neither, so keeping them would make the body
// differ from what is read back and force a replacement on every plan.
func trimBody(v interface{}) string {
	return strings.TrimRight(strings.TrimSpace(v.(string)), whitespace+";")
}
