package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type accountKind int

const (
	accountUnknown accountKind = iota
	accountMissing
	accountRole
	accountUser
)

func resourceRole() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateRole,
		ReadContext:   ReadRole,
		DeleteContext: DeleteRole,
		Importer: &schema.ResourceImporter{
			StateContext: ImportRole,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func CreateRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	roleName := d.Get("name").(string)

	sql := fmt.Sprintf("CREATE ROLE %s", quoteString(roleName))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.Errorf("error creating role: %s", err)
	}

	d.SetId(roleName)

	return nil
}

func ReadRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	roleName := d.Id()

	sql := fmt.Sprintf("SHOW GRANTS FOR %s", quoteString(roleName))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		errorNumber := mysqlErrorNumber(err)
		if errorNumber == nonExistingGrantErrCode || errorNumber == userNotFoundErrCode {
			log.Printf("[WARN] Role (%s) not found; removing from state", roleName)
			d.SetId("")
			return nil
		}
		return diag.Errorf("error reading role %s: %s", roleName, err)
	}

	d.Set("name", roleName)

	return nil
}

func DeleteRole(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	sql := fmt.Sprintf("DROP ROLE %s", quoteString(d.Get("name").(string)))
	log.Printf("[DEBUG] SQL: %s", sql)

	_, err = db.ExecContext(ctx, sql)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// classifyAccount reports whether name is a role or a login user. MariaDB records this in
// mysql.user.is_role; MySQL, Percona and TiDB do not distinguish the two, so we combine the
// state CREATE ROLE leaves behind - locked, no password - with mysql.role_edges, which only
// lists accounts granted to somebody. Returns accountUnknown rather than an error when the
// lookup is unavailable.
func classifyAccount(ctx context.Context, db *sql.DB, name string) accountKind {
	isMariaDB, err := serverMariaDB(db)
	if err != nil {
		log.Printf("[DEBUG] Could not determine server flavor while classifying %s: %v", name, err)
		return accountUnknown
	}

	if isMariaDB {
		// MariaDB stores roles with an empty host, so match on name and prefer the role row.
		var isRole string
		err := db.QueryRowContext(ctx,
			"SELECT is_role FROM mysql.user WHERE user = ? ORDER BY is_role DESC LIMIT 1",
			name).Scan(&isRole)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return accountMissing
		case err != nil:
			log.Printf("[DEBUG] Could not read mysql.user while classifying %s: %v", name, err)
			return accountUnknown
		case isRole == "Y":
			return accountRole
		default:
			return accountUser
		}
	}

	var accountLocked string
	var passwordEmpty int
	err = db.QueryRowContext(ctx,
		"SELECT account_locked, authentication_string = '' FROM mysql.user WHERE user = ? AND host = '%'",
		name).Scan(&accountLocked, &passwordEmpty)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return accountMissing
	case err != nil:
		log.Printf("[DEBUG] Could not read mysql.user while classifying %s: %v", name, err)
		return accountUnknown
	case accountLocked == "Y" && passwordEmpty == 1:
		return accountRole
	}

	// A role later given a password or unlocked still counts; being granted proves it.
	if grantedAsRole(ctx, db, name) {
		return accountRole
	}

	return accountUser
}

// grantedAsRole reports whether name has been granted to another account, which only happens
// to roles. A failed lookup means "no evidence": mysql.role_edges is absent before MySQL 8.
func grantedAsRole(ctx context.Context, db *sql.DB, name string) bool {
	var granted int
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM mysql.role_edges WHERE FROM_USER = ? AND FROM_HOST = '%')",
		name).Scan(&granted)
	if err != nil {
		log.Printf("[DEBUG] Could not read mysql.role_edges while classifying %s: %v", name, err)
		return false
	}
	return granted == 1
}

// ImportRole refuses an account that is really a login user, so importing the wrong name
// fails loudly. Existence is left to ReadRole, which Terraform calls next: clearing the ID
// here would trip the SDK's "missing resource during ImportResourceState" error instead.
func ImportRole(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, err
	}

	roleName := d.Id()

	switch classifyAccount(ctx, db, roleName) {
	case accountUser:
		return nil, fmt.Errorf("%s is a login user, not a role; import it as a mysql_user resource instead", roleName)
	case accountUnknown:
		log.Printf("[WARN] Could not confirm whether %s is a role or a user; importing anyway", roleName)
	}

	d.Set("name", roleName)

	return []*schema.ResourceData{d}, nil
}
