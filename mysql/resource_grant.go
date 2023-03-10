package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type MySQLGrant struct {
	Database   string
	Table      string
	Privileges []string
	Roles      []string
	Grant      bool
}

func resourceGrant() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateGrant,
		UpdateContext: UpdateGrant,
		ReadContext:   ReadGrant,
		DeleteContext: DeleteGrant,
		Importer: &schema.ResourceImporter{
			StateContext: ImportGrant,
		},

		Schema: map[string]*schema.Schema{
			"user": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"role"},
			},

			"role": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"user", "host"},
			},

			"host": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Default:       "localhost",
				ConflictsWith: []string{"role"},
			},

			"database": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"table": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "*",
			},

			"privileges": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"roles": {
				Type:          schema.TypeSet,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"privileges"},
				Elem:          &schema.Schema{Type: schema.TypeString},
				Set:           schema.HashString,
			},

			"grant": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  false,
			},

			"tls_option": {
				Type:       schema.TypeString,
				Optional:   true,
				ForceNew:   true,
				Deprecated: "Please use tls_option in mysql_user.",
				Default:    "NONE",
			},
		},
	}
}

func flattenList(list []interface{}, template string) string {
	var result []string
	for _, v := range list {
		result = append(result, fmt.Sprintf(template, v.(string)))
	}

	return strings.Join(result, ", ")
}

func formatDatabaseName(database string) string {
	if strings.Compare(database, "*") != 0 && !strings.HasSuffix(database, "`") {
		reProcedure := regexp.MustCompile(`(?i)^(function|procedure) (.*)$`)
		if reProcedure.MatchString(database) {
			// This is only a hack - user can specify function / procedure as database.
			database = reProcedure.ReplaceAllString(database, "$1 `${2}`")
		} else {
			database = fmt.Sprintf("`%s`", database)
		}
	}

	return database
}

func formatTableName(table string) string {
	if table == "" || table == "*" {
		return fmt.Sprintf("*")
	}
	return fmt.Sprintf("`%s`", table)
}

// Formats user/host or role. Returns the formatted string and whether it is role. And an error in case it's not supported.
func userOrRole(user string, host string, role string, hasRoles bool) (string, bool, error) {
	if len(user) > 0 && len(host) > 0 {
		return fmt.Sprintf("'%s'@'%s'", user, host), false, nil
	} else if len(role) > 0 {
		if !hasRoles {
			return "", false, fmt.Errorf("roles are only supported on MySQL 8 and above")
		}

		return fmt.Sprintf("'%s'", role), true, nil
	} else {
		return "", false, fmt.Errorf("user with host or a role is required")
	}
}

func supportsRoles(ctx context.Context, meta interface{}) (bool, error) {
	currentVersion := getVersionFromMeta(ctx, meta)

	requiredVersion, _ := version.NewVersion("8.0.0")
	hasRoles := currentVersion.GreaterThan(requiredVersion)
	return hasRoles, nil
}

func CreateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	hasRoles, err := supportsRoles(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}

	var (
		privilegesOrRoles string
		grantOn           string
	)

	hasPrivs := false
	rolesGranted := 0
	if attr, ok := d.GetOk("privileges"); ok {
		privilegesOrRoles = flattenList(attr.(*schema.Set).List(), "%s")
		hasPrivs = true
	} else if attr, ok := d.GetOk("roles"); ok {
		if !hasRoles {
			return diag.Errorf("Roles are only supported on MySQL 8 and above")
		}
		listOfRoles := attr.(*schema.Set).List()
		rolesGranted = len(listOfRoles)
		privilegesOrRoles = flattenList(listOfRoles, "'%s'")
	} else {
		return diag.Errorf("One of privileges or roles is required")
	}

	user := d.Get("user").(string)
	host := d.Get("host").(string)
	role := d.Get("role").(string)

	userOrRole, isRole, err := userOrRole(user, host, role, hasRoles)
	if err != nil {
		return diag.Errorf("failed getting whether it's user or a role: %v", err)
	}
	database := d.Get("database").(string)
	table := d.Get("table").(string)

	grants, err := showGrants(ctx, db, userOrRole, database, table)
	if err != nil {
		return diag.Errorf("failed showing grants: %v", err)
	}

	for _, grant := range grants {
		if len(grant.Privileges) == 0 {
			continue
		}

		if hasPrivs {
			if grant.Database == database && grant.Table == table {
				return diag.Errorf("user/role %s already has unmanaged grant to %s.%s - import it first", userOrRole, grant.Database, grant.Table)
			}
		} else {
			// Granting role is just role without DB & table.
			if grant.Database == "" && grant.Table == "" {
				return diag.Errorf("user/role %s already has unmanaged grant for roles %v - import it first", userOrRole, grant.Roles)
			}
		}
	}

	// DB and table have to be wrapped in backticks in some cases.
	databaseWrapped := formatDatabaseName(database)
	tableWrapped := formatTableName(table)
	if (!isRole || hasPrivs) && rolesGranted == 0 {
		grantOn = fmt.Sprintf(" ON %s.%s", databaseWrapped, tableWrapped)
	}

	stmtSQL := fmt.Sprintf("GRANT %s%s TO %s",
		privilegesOrRoles,
		grantOn,
		userOrRole)

	// MySQL 8+ doesn't allow REQUIRE on a GRANT statement.
	if !hasRoles && d.Get("tls_option").(string) != "" && strings.ToLower(d.Get("tls_option").(string)) != "none" {
		stmtSQL += fmt.Sprintf(" REQUIRE %s", d.Get("tls_option").(string))
	}

	if d.Get("grant").(bool) {
		if rolesGranted == 0 {
			stmtSQL += " WITH GRANT OPTION"
		}
		// TODO: consider WITH ADMIN OPTION here.
		// However, there is no obvious way to revoke it, so not adding it here.
	}

	log.Println("Executing statement:", stmtSQL)
	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("Error running SQL (%s): %s", stmtSQL, err)
	}

	id := fmt.Sprintf("%s@%s:%s", user, host, databaseWrapped)
	if isRole {
		id = fmt.Sprintf("%s:%s", role, databaseWrapped)
	}

	d.SetId(id)

	return ReadGrant(ctx, d, meta)
}

func ReadGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting database from Meta: %v", err)
	}

	hasRoles, err := supportsRoles(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}
	userOrRole, _, err := userOrRole(
		d.Get("user").(string),
		d.Get("host").(string),
		d.Get("role").(string),
		hasRoles)
	if err != nil {
		return diag.Errorf("failed getting user or role: %v", err)
	}
	database := d.Get("database").(string)
	table := d.Get("table").(string)
	rolesSet := d.Get("roles").(*schema.Set)
	rolesCount := len(rolesSet.List())

	var grants []*MySQLGrant
	if rolesCount != 0 {
		// For some reason, role can have still database / table, that
		// makes no sense. Remove them when reading.
		database = ""
		table = ""
	}
	grants, err = showGrants(ctx, db, userOrRole, database, table)

	if err != nil {
		return diag.Errorf("error reading grant for %s: %v", userOrRole, err)
	}

	if len(grants) == 0 {
		log.Printf("[WARN] GRANT not found for %s (%s) - removing from state", userOrRole, err)
		d.SetId("")
		return nil
	}

	var privileges []string
	var roles []string
	var grantOption bool

	for _, grant := range grants {
		if grant.Database == database && grant.Table == table {
			privileges = makePrivs(setToArray(d.Get("privileges")), grant.Privileges)
		}
		// Granting role is just role without DB & table.
		if grant.Database == "" && grant.Table == "" {
			roles = grant.Roles
		}

		if grant.Grant {
			grantOption = true
		}
	}

	d.Set("privileges", privileges)
	d.Set("roles", roles)
	d.Set("grant", grantOption)

	return nil
}

func UpdateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	hasRoles, err := supportsRoles(ctx, meta)

	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}

	userOrRole, _, err := userOrRole(
		d.Get("user").(string),
		d.Get("host").(string),
		d.Get("role").(string),
		hasRoles)

	if err != nil {
		return diag.Errorf("failed getting user or role: %v", err)
	}

	database := d.Get("database").(string)
	table := d.Get("table").(string)

	if d.HasChange("privileges") {
		err = updatePrivileges(ctx, d, db, userOrRole, database, table)
		if err != nil {
			return diag.Errorf("failed updating privileges: %v", err)
		}
	}

	return nil
}

func updatePrivileges(ctx context.Context, d *schema.ResourceData, db *sql.DB, user string, database string, table string) error {
	oldPrivsIf, newPrivsIf := d.GetChange("privileges")
	oldPrivs := oldPrivsIf.(*schema.Set)
	newPrivs := newPrivsIf.(*schema.Set)
	grantIfs := newPrivs.Difference(oldPrivs).List()
	revokeIfs := oldPrivs.Difference(newPrivs).List()

	if len(revokeIfs) > 0 {
		revokes := make([]string, len(revokeIfs))

		for i, v := range revokeIfs {
			revokes[i] = v.(string)
		}

		sql := fmt.Sprintf("REVOKE %s ON %s.%s FROM %s", strings.Join(revokes, ","), formatDatabaseName(database), formatTableName(table), user)

		log.Printf("[DEBUG] SQL: %s", sql)

		if _, err := db.ExecContext(ctx, sql); err != nil {
			return err
		}
	}

	if len(grantIfs) > 0 {
		grants := make([]string, len(grantIfs))

		for i, v := range grantIfs {
			grants[i] = v.(string)
		}

		sql := fmt.Sprintf("GRANT %s ON %s.%s TO %s", strings.Join(grants, ","), formatDatabaseName(database), formatTableName(table), user)

		log.Printf("[DEBUG] SQL: %s", sql)

		if _, err := db.ExecContext(ctx, sql); err != nil {
			return err
		}
	}

	return nil
}

func DeleteGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	database := formatDatabaseName(d.Get("database").(string))
	table := formatTableName(d.Get("table").(string))

	hasRoles, err := supportsRoles(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}

	userOrRole, _, err := userOrRole(
		d.Get("user").(string),
		d.Get("host").(string),
		d.Get("role").(string),
		hasRoles)
	if err != nil {
		return diag.Errorf("failed getting user or role: %v", err)
	}

	roles := d.Get("roles").(*schema.Set)
	rolesCount := len(roles.List())

	privileges := d.Get("privileges").(*schema.Set)

	if rolesCount == 0 {
		sqlStatement := fmt.Sprintf("REVOKE GRANT OPTION ON %s.%s FROM %s",
			database,
			table,
			userOrRole)

		log.Printf("[DEBUG] SQL: %s", sqlStatement)
		_, err = db.ExecContext(ctx, sqlStatement)
		if err != nil {
			if !isNonExistingGrant(err) {
				return diag.Errorf("error revoking GRANT (%s): %s", sqlStatement, err)
			}
		}
	}

	whatToRevoke := fmt.Sprintf("ALL ON %s.%s", database, table)
	if len(roles.List()) > 0 {
		whatToRevoke = flattenList(roles.List(), "'%s'")
	} else if len(privileges.List()) > 0 {
		privilegeList := flattenList(privileges.List(), "%s")
		whatToRevoke = fmt.Sprintf("%s ON %s.%s", privilegeList, database, table)
	}

	sqlStatement := fmt.Sprintf("REVOKE %s FROM %s", whatToRevoke, userOrRole)
	log.Printf("[DEBUG] SQL: %s", sqlStatement)
	_, err = db.ExecContext(ctx, sqlStatement)
	if err != nil {
		if !isNonExistingGrant(err) {
			return diag.Errorf("error revoking ALL (%s): %s", sqlStatement, err)
		}
	}

	return nil
}

func isNonExistingGrant(err error) bool {
	if driverErr, ok := err.(*mysql.MySQLError); ok {
		// 1141 = ER_NONEXISTING_GRANT
		// 1147 = ER_NONEXISTING_TABLE_GRANT
		// 1403 = ER_NONEXISTING_PROC_GRANT

		if driverErr.Number == 1141 || driverErr.Number == 1147 || driverErr.Number == 1403 {
			return true
		}
	}
	return false
}

func ImportGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	userHostDatabaseTable := strings.SplitN(d.Id(), "@", 4)

	if len(userHostDatabaseTable) != 4 {
		return nil, fmt.Errorf("wrong ID format %s - expected user@host@database@table where some parts can be empty)", d.Id())
	}

	user := userHostDatabaseTable[0]
	host := userHostDatabaseTable[1]
	database := userHostDatabaseTable[2]
	table := userHostDatabaseTable[3]

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, err
	}

	grants, err := showGrants(ctx, db, fmt.Sprintf("'%s'@'%s'", user, host), database, table)

	if err != nil {
		return nil, err
	}

	results := []*schema.ResourceData{}

	for _, grant := range grants {
		results = append(results, restoreGrant(user, host, grant))
	}

	return results, nil
}

func restoreGrant(user string, host string, grant *MySQLGrant) *schema.ResourceData {
	d := resourceGrant().Data(nil)

	database := grant.Database
	id := fmt.Sprintf("%s@%s:%s", user, host, formatDatabaseName(database))
	d.SetId(id)

	d.Set("user", user)
	d.Set("host", host)
	d.Set("database", database)
	d.Set("table", grant.Table)
	d.Set("grant", grant.Grant)
	d.Set("tls_option", "NONE")
	d.Set("privileges", grant.Privileges)

	return d
}

func showGrants(ctx context.Context, db *sql.DB, user, database, table string) ([]*MySQLGrant, error) {
	allGrants, err := showUserGrants(ctx, db, user)
	if err != nil {
		return nil, fmt.Errorf("showGrants - getting all grants failed: %w", err)
	}
	grants := []*MySQLGrant{}
	for _, grant := range allGrants {
		if grant.Database == database && grant.Table == table {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

func showUserGrants(ctx context.Context, db *sql.DB, user string) ([]*MySQLGrant, error) {
	grants := []*MySQLGrant{}

	sqlStatement := fmt.Sprintf("SHOW GRANTS FOR %s", user)
	log.Printf("[DEBUG] SQL: %s", sqlStatement)
	rows, err := db.QueryContext(ctx, sqlStatement)

	if isNonExistingGrant(err) {
		return []*MySQLGrant{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("showUserGrants - getting grants failed: %w", err)
	}

	defer rows.Close()
	re := regexp.MustCompile(`^GRANT (.+) ON (.+?)\.(.+?) TO ([^ ]+)`)

	// Ex: GRANT `app_read`@`%`,`app_write`@`%` TO `rw_user1`@`localhost
	reRole := regexp.MustCompile(`^GRANT (.+) TO`)
	reGrant := regexp.MustCompile(`\bGRANT OPTION\b`)

	for rows.Next() {
		var rawGrant string

		err := rows.Scan(&rawGrant)
		if err != nil {
			return nil, fmt.Errorf("showUserGrants - reading row failed: %w", err)
		}

		if strings.HasPrefix(rawGrant, "REVOKE") {
			log.Printf("[WARN] Partial revokes are not fully supported and lead to unexpected behavior. Consult documentation https://dev.mysql.com/doc/refman/8.0/en/partial-revokes.html on how to disable them for safe and reliable terraform. Relevant partial revoke: %s\n", rawGrant)
			continue
		}

		if m := re.FindStringSubmatch(rawGrant); len(m) == 5 {
			privsStr := m[1]
			privList := extractPermTypes(privsStr)
			privileges := make([]string, len(privList))

			for i, priv := range privList {
				privileges[i] = strings.TrimSpace(priv)
			}
			grantUserHost := m[4]
			if normalizeUserHost(grantUserHost) != normalizeUserHost(user) {
				// Percona returns also grants for % if we requested IP.
				// Skip them as we don't want terraform to consider it.
				log.Printf("[DEBUG] Skipping grant with host %v while we want %v", grantUserHost, user)
				continue
			}

			grant := &MySQLGrant{
				Database:   strings.Trim(m[2], "`\""),
				Table:      strings.Trim(m[3], "`\""),
				Privileges: privileges,
				Grant:      reGrant.MatchString(rawGrant),
			}

			if len(privileges) > 0 {
				grants = append(grants, grant)
			}

		} else if m := reRole.FindStringSubmatch(rawGrant); len(m) == 2 {
			roleStr := m[1]
			rolesStart := strings.Split(roleStr, ",")
			roles := make([]string, len(rolesStart))

			for i, role := range rolesStart {
				roles[i] = strings.Trim(role, "`@%\" ")
			}

			grant := &MySQLGrant{
				Roles: roles,
				Grant: reGrant.MatchString(rawGrant),
			}

			grants = append(grants, grant)
		} else {
			return nil, fmt.Errorf("failed to parse grant statement: %s", rawGrant)
		}
	}

	log.Printf("[DEBUG] Parsed grants are: %v", grants)
	return grants, nil
}

func normalizeUserHost(userHost string) string {
	if !strings.Contains(userHost, "@") {
		userHost = fmt.Sprint(userHost, "@%")
	}
	withoutQuotes := strings.ReplaceAll(userHost, "'", "")
	withoutBackticks := strings.ReplaceAll(withoutQuotes, "`", "")
	withoutDblQuotes := strings.ReplaceAll(withoutBackticks, "\"", "")
	return withoutDblQuotes
}

func removeUselessPerms(grants []string) []string {
	ret := []string{}
	for _, grant := range grants {
		if grant != "USAGE" {
			ret = append(ret, grant)
		}
	}
	return ret
}

func extractPermTypes(g string) []string {
	grants := []string{}

	inParentheses := false
	currentWord := []rune{}
	for _, b := range g {
		switch b {
		case ',':
			if inParentheses {
				currentWord = append(currentWord, b)
			} else {
				grants = append(grants, string(currentWord))
				currentWord = []rune{}
			}
			break
		case '(':
			inParentheses = true
			currentWord = append(currentWord, b)
			break
		case ')':
			inParentheses = false
			currentWord = append(currentWord, b)
			break
		default:
			if unicode.IsSpace(b) && len(currentWord) == 0 {
				break
			}
			currentWord = append(currentWord, b)
		}
	}
	grants = append(grants, string(currentWord))
	return removeUselessPerms(grants)
}

func normalizeColumnOrder(perm string) string {
	re := regexp.MustCompile("^([^(]*)\\((.*)\\)$")
	// We may get inputs like
	// 	SELECT(b,a,c)   -> SELECT(a,b,c)
	// 	DELETE          -> DELETE
	// if it's without parentheses, return it right away.
	// Else split what is inside, sort it, concat together and return the result.
	m := re.FindStringSubmatch(perm)
	if m == nil || len(m) < 3 {
		return perm
	}

	parts := strings.Split(m[2], ",")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], "` ")
	}
	sort.Strings(parts)
	partsTogether := strings.Join(parts, ", ")
	return fmt.Sprintf("%s(%s)", m[1], partsTogether)
}

func normalizePerms(perms []string) []string {
	// Spaces and backticks are optional, let's ignore them.
	re := regexp.MustCompile("[ `]")
	ret := []string{}
	for _, perm := range perms {
		permNorm := re.ReplaceAllString(perm, "")
		permUcase := strings.ToUpper(permNorm)
		if permUcase == "ALL" || permUcase == "ALLPRIVILEGES" {
			permUcase = "ALL PRIVILEGES"
		}
		permSortedColumns := normalizeColumnOrder(permUcase)

		ret = append(ret, permSortedColumns)
	}
	return ret
}

func makePrivs(have, want []string) []string {
	// This is tricky to prevent diffs that cannot be suppressed easily.
	// Example:
	// Have select(`c1`, `c2`), insert (c3,c2)
	// Want select(c2,c1), insert(c3,c2)
	// So we want to normalize both and then go from "want" to "have" to
	// We'll have map want->wantnorm = havenorm->have

	// Also, we need to return all mapped values of "want".

	// After normalize, same indices have the same values, prepare maps.
	haveNorm := normalizePerms(have)
	haveNormToHave := map[string]string{}
	for i := range haveNorm {
		haveNormToHave[haveNorm[i]] = have[i]
	}

	wantNorm := normalizePerms(want)
	wantNormToWant := map[string]string{}
	for i := range wantNorm {
		wantNormToWant[want[i]] = wantNorm[i]
	}

	retSet := []string{}
	for _, w := range want {
		suspect := haveNormToHave[wantNormToWant[w]]
		if suspect == "" {
			// Nothing found in what we have.
			retSet = append(retSet, w)
		} else {
			retSet = append(retSet, suspect)
		}
	}

	return retSet
}

func setToArray(s interface{}) []string {
	set, ok := s.(*schema.Set)
	if !ok {
		return []string{}
	}

	ret := []string{}
	for _, elem := range set.List() {
		ret = append(ret, elem.(string))
	}
	return ret
}
