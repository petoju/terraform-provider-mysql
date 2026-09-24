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
				ForceNew:     true,
				StateFunc:    normalizeDefiner,
				ValidateFunc: validateDefiner,
			},

			"parameter": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"mode": {
							Type:         schema.TypeString,
							Optional:     true,
							ForceNew:     true,
							Default:      "IN",
							StateFunc:    upperCase,
							ValidateFunc: validation.StringInSlice(procedureParameterModes, true),
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
					},
				},
			},

			"body": {
				Type:      schema.TypeString,
				Required:  true,
				ForceNew:  true,
				StateFunc: trimSpace,
			},

			"comment": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},

			"deterministic": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
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

	stmtSQL, err := procedureCreateSQL(d)
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

	// Only the characteristics below can be altered in place; everything else
	// is ForceNew because MySQL requires dropping and recreating the routine.
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
		database, name, err := parseProcedureID(d.Id())
		if err != nil {
			return diag.FromErr(err)
		}

		stmtSQL := fmt.Sprintf("ALTER PROCEDURE %s %s", qualifiedProcedureName(database, name), strings.Join(characteristics, " "))
		log.Println("[DEBUG] Executing statement:", stmtSQL)

		if _, err := db.ExecContext(ctx, stmtSQL); err != nil {
			return diag.Errorf("failed updating procedure: %v", err)
		}
	}

	return ReadProcedure(ctx, d, meta)
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

	stmtSQL := "SELECT ROUTINE_DEFINITION, ROUTINE_COMMENT, IS_DETERMINISTIC, SQL_DATA_ACCESS, SECURITY_TYPE, DEFINER " +
		"FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'"
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	var body, comment, definer sql.NullString
	var isDeterministic, sqlDataAccess, securityType string
	err = db.QueryRowContext(ctx, stmtSQL, database, name).Scan(
		&body, &comment, &isDeterministic, &sqlDataAccess, &securityType, &definer,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed reading procedure: %w", err)
	}

	parameters, err := readProcedureParameters(ctx, db, database, name)
	if err != nil {
		if mysqlErrorNumber(err) == unknownProcedureErrCode {
			d.SetId("")
			return nil
		}
		return err
	}

	d.Set("database", database)
	d.Set("name", name)
	d.Set("body", body.String)
	d.Set("comment", comment.String)
	d.Set("deterministic", isDeterministic == "YES")
	d.Set("sql_data_access", sqlDataAccess)
	d.Set("security_type", securityType)
	d.Set("definer", normalizeDefiner(definer.String))
	d.Set("parameter", parameters)

	return nil
}

// readProcedureParameters recovers the parameter list from SHOW CREATE
// PROCEDURE. information_schema.PARAMETERS cannot be used because it reports
// the type as MySQL normalized it (INT becomes int(11) on 5.7 and MariaDB but
// int on 8.0), which would produce a permanent diff. SHOW CREATE PROCEDURE
// returns the parameter list exactly as it was declared.
func readProcedureParameters(ctx context.Context, db *sql.DB, database, name string) ([]interface{}, error) {
	stmtSQL := "SHOW CREATE PROCEDURE " + qualifiedProcedureName(database, name)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	createStmt, err := showCreateProcedure(ctx, db, stmtSQL)
	if err != nil {
		return nil, err
	}

	parameterList, err := extractProcedureParameterList(createStmt)
	if err != nil {
		return nil, fmt.Errorf("failed parsing %q: %w", createStmt, err)
	}

	return parseProcedureParameterList(parameterList)
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

func procedureCreateSQL(d *schema.ResourceData) (string, error) {
	parameters, err := procedureParametersSQL(d.Get("parameter").([]interface{}))
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString("CREATE ")

	if definer := d.Get("definer").(string); definer != "" {
		user, host, err := splitDefiner(definer)
		if err != nil {
			return "", err
		}
		builder.WriteString(fmt.Sprintf("DEFINER = %s ", formatUserIdentifier(user, host)))
	}

	builder.WriteString(fmt.Sprintf(
		"PROCEDURE %s (%s)",
		qualifiedProcedureName(d.Get("database").(string), d.Get("name").(string)),
		parameters,
	))

	if comment := d.Get("comment").(string); comment != "" {
		builder.WriteString("\nCOMMENT " + quoteString(comment))
	}

	if d.Get("deterministic").(bool) {
		builder.WriteString("\nDETERMINISTIC")
	} else {
		builder.WriteString("\nNOT DETERMINISTIC")
	}

	builder.WriteString("\n" + strings.ToUpper(d.Get("sql_data_access").(string)))
	builder.WriteString("\nSQL SECURITY " + strings.ToUpper(d.Get("security_type").(string)))
	builder.WriteString("\n" + strings.TrimSpace(d.Get("body").(string)))

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
// follow the routine name in a CREATE PROCEDURE statement.
func extractProcedureParameterList(createStmt string) (string, error) {
	start := -1
	depth := 0

	for i := 0; i < len(createStmt); i++ {
		switch c := createStmt[i]; c {
		case '`', '\'', '"':
			end, err := skipQuoted(createStmt, i)
			if err != nil {
				return "", err
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
				return "", fmt.Errorf("unbalanced parentheses")
			}
			if depth == 0 {
				return createStmt[start:i], nil
			}
		}
	}

	return "", fmt.Errorf("no parameter list found")
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

func trimSpace(v interface{}) string {
	return strings.TrimSpace(v.(string))
}
