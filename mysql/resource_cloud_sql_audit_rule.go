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

func resourceCloudSQLAuditRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateCloudSQLAuditRule,
		ReadContext:   ReadCloudSQLAuditRule,
		DeleteContext: DeleteCloudSQLAuditRule,
		Importer: &schema.ResourceImporter{
			StateContext: ImportCloudSQLAuditRule,
		},

		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"database": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"object": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"operation": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"op_result": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					"S",
					"U",
					"B",
				}, false),
			},
		},
	}
}

func CreateCloudSQLAuditRule(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	cloudSQLAuditRuleUser := getCommaSeparatedList(d, "username")
	cloudSQLAuditRuleDb := getCommaSeparatedList(d, "database")
	cloudSQLAuditRuleObj := getCommaSeparatedList(d, "object")
	cloudSQLAuditRuleOps := getCommaSeparatedList(d, "operation")
	cloudSQLAuditRuleOpResult := d.Get("op_result").(string)

	conn, err := db.Conn(ctx)
	if err != nil {
		return diag.Errorf("error getting database connection: %v", err)
	}
	defer conn.Close()

	query := "CALL mysql.cloudsql_create_audit_rule(?, ?, ?, ?, ?, 1, @outval, @outmsg)"
	log.Printf(
		"[DEBUG] SQL: %s | params: %q, %q, %q, %q, %q",
		query,
		cloudSQLAuditRuleUser,
		cloudSQLAuditRuleDb,
		cloudSQLAuditRuleObj,
		cloudSQLAuditRuleOps,
		cloudSQLAuditRuleOpResult,
	)

	_, err = conn.ExecContext(
		ctx,
		query,
		cloudSQLAuditRuleUser,
		cloudSQLAuditRuleDb,
		cloudSQLAuditRuleObj,
		cloudSQLAuditRuleOps,
		cloudSQLAuditRuleOpResult,
	)
	if err != nil {
		return diag.Errorf("error creating audit rule: %v", err)
	}

	query = "SELECT @outval, @outmsg;"

	log.Printf("[DEBUG] SQL: %s", query)

	var outval int
	var outmsg sql.NullString
	err = conn.QueryRowContext(ctx, query).Scan(&outval, &outmsg)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return diag.Errorf("error creating audit rule: query returned no rows")
		}

		return diag.Errorf("error reading outval for creating audit rule: %v", err)
	}

	if outval != 0 {
		return diag.Errorf("error creating audit rule (error code %d): %s", outval, outmsg.String)
	}

	query = "CALL mysql.cloudsql_list_audit_rule('*', @outval, @outmsg);"
	log.Printf("[DEBUG] SQL: %s", query)

	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return diag.Errorf("error reading audit rules from DB: %v", err)
	}
	defer rows.Close()

	var auditRule int
	for rows.Next() {
		var id int
		var username string
		var dbname string
		var object string
		var operation string
		var op_result string
		err := rows.Scan(&id, &username, &dbname, &object, &operation, &op_result)
		if err != nil {
			return diag.Errorf("error scanning audit rules: %v", err)
		}
		if username == cloudSQLAuditRuleUser && dbname == cloudSQLAuditRuleDb && operation == cloudSQLAuditRuleOps && op_result == cloudSQLAuditRuleOpResult && object == cloudSQLAuditRuleObj {
			auditRule = id
			break
		}
	}

	if rows.Err() != nil {
		return diag.Errorf("error getting rows: %v", rows.Err())
	}

	if auditRule == 0 {
		return diag.Errorf("failed to find audit rule after creation")
	}

	d.SetId(fmt.Sprintf("%d", auditRule))

	return nil
}

func ReadCloudSQLAuditRule(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	err = readAuditRule(db, ctx, d)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func DeleteCloudSQLAuditRule(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return diag.Errorf("error getting database connection: %v", err)
	}
	defer conn.Close()

	query := "CALL mysql.cloudsql_delete_audit_rule(?, 1, @outval, @outmsg);"
	log.Printf("[DEBUG] SQL: %s | params: %q", query, d.Id())

	_, err = conn.ExecContext(ctx, query, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	query = "SELECT @outval, @outmsg;"

	log.Printf("[DEBUG] SQL: %s", query)

	var outval int
	var outmsg sql.NullString
	err = conn.QueryRowContext(ctx, query).Scan(&outval, &outmsg)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return diag.Errorf("error deleting audit rule: query returned no rows")
		}

		return diag.Errorf("error reading outval for deleting audit rule: %v", err)
	}

	if outval != 0 {
		return diag.Errorf("error deleting audit rule (error code %d): %s", outval, outmsg.String)
	}

	return nil
}

func ImportCloudSQLAuditRule(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, err
	}

	err = readAuditRule(db, ctx, d)
	if err != nil {
		return nil, err
	}

	return []*schema.ResourceData{d}, nil
}

func readAuditRule(db *sql.DB, ctx context.Context, d *schema.ResourceData) error {
	query := "CALL mysql.cloudsql_list_audit_rule(?, @outval, @outmsg);"
	log.Printf("[DEBUG] SQL: %s | params: %q", query, d.Id())

	rows, err := db.QueryContext(ctx, query, d.Id())
	if err != nil {
		return fmt.Errorf("failed to read audit rules from DB: %v", err)
	}
	defer rows.Close()

	var id int
	var username string
	var dbname string
	var object string
	var operation string
	var op_result string
	if rows.Next() {
		err := rows.Scan(&id, &username, &dbname, &object, &operation, &op_result)
		if err != nil {
			return fmt.Errorf("failed scanning audit rules: %v", err)
		}

		usernameList, err := splitList(username)
		if err != nil {
			return fmt.Errorf("failed splitting username list: %v", err)
		}
		d.Set("username", usernameList)

		dbnameList, err := splitList(dbname)
		if err != nil {
			return fmt.Errorf("failed splitting database list: %v", err)
		}
		d.Set("database", dbnameList)

		objectList, err := splitList(object)
		if err != nil {
			return fmt.Errorf("failed splitting object list: %v", err)
		}
		d.Set("object", objectList)

		operationList, err := splitList(operation)
		if err != nil {
			return fmt.Errorf("failed splitting operation list: %v", err)
		}
		d.Set("operation", operationList)

		d.Set("op_result", op_result)
	} else {
		d.SetId("")
	}

	if rows.Err() != nil {
		return fmt.Errorf("failed getting rows: %v", rows.Err())
	}
	return nil
}

func getCommaSeparatedList(d *schema.ResourceData, key string) string {
	raw := d.Get(key).([]interface{})

	values := make([]string, len(raw))
	for i, item := range raw {
		values[i] = item.(string)
	}

	return strings.Join(values, ",")
}

func splitList(input string) ([]interface{}, error) {
	var result []interface{}
	start := 0
	inBackticks := false

	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '`':
			if inBackticks && i+1 < len(input) && input[i+1] == '`' {
				// Escaped backtick: preserve both characters.
				i++
				continue
			}

			inBackticks = !inBackticks

		case ',':
			if !inBackticks {
				name := strings.TrimSpace(input[start:i])
				if name == "" {
					return nil, fmt.Errorf("empty entry in list")
				}

				result = append(result, name)
				start = i + 1
			}
		}
	}

	if inBackticks {
		return nil, fmt.Errorf("unterminated backtick-quoted in list")
	}

	name := strings.TrimSpace(input[start:])
	if name == "" {
		return nil, fmt.Errorf("empty entry in list")
	}

	result = append(result, name)

	return result, nil
}
