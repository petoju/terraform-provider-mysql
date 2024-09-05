package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const defaultCharacterSetKeyword = "CHARACTER SET "
const defaultCollateKeyword = "COLLATE "
const placementPolicyKeyword = "PLACEMENT POLICY="
const placementPolicyDefault = "default"
const unknownDatabaseErrCode = 1049

var placementPolicyRegex = regexp.MustCompile(fmt.Sprintf("%s`([a-zA-Z0-9_-]+)`", placementPolicyKeyword))

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

			"placement_policy": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
		},
	}
}

func CreateDatabase(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	stmtSQL, err := databaseConfigSQL("CREATE", d, db)
	if err != nil {
		return diag.Errorf("failed constructing create SQL statement: %v", err)
	}
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

	stmtSQL, err := databaseConfigSQL("ALTER", d, db)
	if err != nil {
		return diag.Errorf("failed constructing update SQL statement: %v", err)
	}
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
	placementPolicyMatches := placementPolicyRegex.FindStringSubmatch(createSQL)

	placementPolicy := ""
	if len(placementPolicyMatches) >= 2 {
		placementPolicy = placementPolicyMatches[1]
	}

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
	d.Set("placement_policy", placementPolicy)

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

func databaseConfigSQL(verb string, d *schema.ResourceData, db *sql.DB) (string, error) {
	name := d.Get("name").(string)
	defaultCharset := d.Get("default_character_set").(string)
	defaultCollation := d.Get("default_collation").(string)
	placementPolicy := d.Get("placement_policy").(string)

	var defaultCharsetClause string
	var defaultCollationClause string
	var placementPolicyClause string

	if defaultCharset != "" {
		defaultCharsetClause = defaultCharacterSetKeyword + quoteIdentifier(defaultCharset)
	}
	if defaultCollation != "" {
		defaultCollationClause = defaultCollateKeyword + quoteIdentifier(defaultCollation)
	}

	isTiDB, _, _, err := serverTiDB(db)
	if err != nil {
		return "", err
	}

	if isTiDB {
		if placementPolicy != "" {
			placementPolicyClause = placementPolicyKeyword + quoteIdentifier(placementPolicy)
		} else {
			placementPolicyClause = placementPolicyKeyword + quoteIdentifier(placementPolicyDefault)
		}
	} else if placementPolicy != "" {
		return "", fmt.Errorf("placement_policy is only supported for TiDB")
	}

	return fmt.Sprintf(
		"%s DATABASE %s %s %s %s",
		verb,
		quoteIdentifier(name),
		defaultCharsetClause,
		defaultCollationClause,
		placementPolicyClause,
	), nil
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
	err := ReadDatabase(ctx, d, meta)
	if err != nil {
		return nil, fmt.Errorf("error while importing: %v", err)
	}

	return []*schema.ResourceData{d}, nil
}
