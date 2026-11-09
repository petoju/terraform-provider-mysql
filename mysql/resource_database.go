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
)

const defaultCharacterSetKeyword = "CHARACTER SET "
const defaultCollateKeyword = "COLLATE "
const unknownDatabaseErrCode = 1049

func resourceDatabase() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateDatabase,
		UpdateContext: UpdateDatabase,
		ReadContext:   ReadDatabase,
		DeleteContext: DeleteDatabase,
		Importer: &schema.ResourceImporter{
			StateContext: ImportDatabase,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"default_character_set": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "utf8mb4",
			},

			"default_collation": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "utf8mb4_general_ci",
			},
		},
	}
}

func CreateDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL := databaseConfigSQL("CREATE", d)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed running SQL to create DB: %v", err)
	}

	d.SetId(d.Get("name").(string))

	return ReadDatabase(ctx, d, meta)
}

func UpdateDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL := databaseConfigSQL("ALTER", d)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed updating DB: %v", err)
	}

	return ReadDatabase(ctx, d, meta)
}

func ReadDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// This is kinda flimsy-feeling, since it depends on the formatting
	// of the SHOW CREATE DATABASE output... but this data doesn't seem
	// to be available any other way, so hopefully MySQL keeps this
	// compatible in future releases.

	name := d.Id()
	stmtSQL := "SHOW CREATE DATABASE " + quoteIdentifier(name)

	log.Println("[DEBUG] Executing query:", stmtSQL)
	var createSQL, _database string
	err = db.QueryRowContext(ctx, stmtSQL).Scan(&_database, &createSQL)
	if err != nil {
		if mysqlErrorNumber(err) == unknownDatabaseErrCode {
			d.SetId("")
			return nil
		}
		return diag.Errorf("Error during show create database: %s", err)
	}

	defaultCharset := extractIdentAfter(createSQL, defaultCharacterSetKeyword)
	defaultCollation := extractIdentAfter(createSQL, defaultCollateKeyword)

	if defaultCollation == "" && defaultCharset != "" {
		// MySQL doesn't return the collation if it's the default one for
		// the charset, so if we don't have a collation we need to go
		// hunt for the default.
		stmtSQL := "SELECT COLLATION_NAME, CHARACTER_SET_NAME FROM INFORMATION_SCHEMA.COLLATIONS WHERE CHARACTER_SET_NAME = ? AND `IS_DEFAULT` = 'Yes';"
		/*
			Mysql (5.7, 8.0), TiDB (6.x, 7.x) example:
			> SELECT COLLATION_NAME, CHARACTER_SET_NAME FROM INFORMATION_SCHEMA.COLLATIONS WHERE CHARACTER_SET_NAME = 'utf8mb4' AND `IS_DEFAULT` = 'Yes';

					+--------------------+--------------------+
					| COLLATION_NAME     | CHARACTER_SET_NAME |
					+--------------------+--------------------+
					| utf8mb4_0900_ai_ci | utf8mb4            |
					+--------------------+--------------------+


		*/
		var empty interface{}

		res := db.QueryRow(stmtSQL, defaultCharset).Scan(&defaultCollation, &empty)

		if res != nil {
			if errors.Is(res, sql.ErrNoRows) {
				return diag.Errorf("charset %s has no default collation", defaultCharset)
			}

			return diag.Errorf("error getting default charset: %s, %s", res, defaultCharset)
		}
	}

	d.Set("name", name)
	d.Set("default_character_set", defaultCharset)
	d.Set("default_collation", defaultCollation)

	return nil
}

func DeleteDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	name := d.Id()
	stmtSQL := "DROP DATABASE " + quoteIdentifier(name)
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("failed deleting DB: %v", err)
	}

	d.SetId("")
	return nil
}

func databaseConfigSQL(verb string, d *schema.ResourceData) string {
	name := d.Get("name").(string)
	defaultCharset := d.Get("default_character_set").(string)
	defaultCollation := d.Get("default_collation").(string)

	var defaultCharsetClause string
	var defaultCollationClause string

	if defaultCharset != "" {
		defaultCharsetClause = defaultCharacterSetKeyword + quoteIdentifier(defaultCharset)
	}
	if defaultCollation != "" {
		defaultCollationClause = defaultCollateKeyword + quoteIdentifier(defaultCollation)
	}

	return fmt.Sprintf(
		"%s DATABASE %s %s %s",
		verb,
		quoteIdentifier(name),
		defaultCharsetClause,
		defaultCollationClause,
	)
}

func extractIdentAfter(sql string, keyword string) string {
	charsetIndex := strings.Index(sql, keyword)
	if charsetIndex != -1 {
		charsetIndex += len(keyword)
		remain := sql[charsetIndex:]
		spaceIndex := strings.IndexRune(remain, ' ')
		return remain[:spaceIndex]
	}

	return ""
}

func ImportDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	name := d.Id()
	diags := ReadDatabase(ctx, d, meta)
	if diags.HasError() {
		return nil, fmt.Errorf("error while importing: %v", diags)
	}

	// ReadDatabase clears the ID when the database does not exist. Returning
	// a resource without an ID makes Terraform report a provider bug, so
	// return a proper error instead.
	if d.Id() == "" {
		return nil, fmt.Errorf("database %s not found", name)
	}

	return []*schema.ResourceData{d}, nil
}
