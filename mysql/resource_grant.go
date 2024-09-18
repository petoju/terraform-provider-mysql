package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ObjectT string

var (
	kProcedure ObjectT = "PROCEDURE"
	kFunction  ObjectT = "FUNCTION"
	kTable     ObjectT = "TABLE"
)

var grantCreateMutex = NewKeyedMutex()

type MySQLGrant interface {
	GetId() string
	SQLGrantStatement() string
	SQLRevokeStatement() string
	GetUserOrRole() UserOrRole
	GrantOption() bool

	ConflictsWithGrant(otherGrant MySQLGrant) bool
}

type MySQLGrantWithDatabase interface {
	MySQLGrant
	GetDatabase() string
}

type MySQLGrantWithTable interface {
	MySQLGrantWithDatabase
	GetTable() string
}

type MySQLGrantWithPrivileges interface {
	MySQLGrant
	GetPrivileges() []string
	AppendPrivileges([]string)
}

type MySQLGrantWithRoles interface {
	MySQLGrant
	GetRoles() []string
	AppendRoles([]string)
}

type PrivilegesPartiallyRevocable interface {
	SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string
}

type UserOrRole struct {
	Name string
	Host string
}

func (u UserOrRole) IDString() string {
	if u.Host == "" {
		return u.Name
	}
	return fmt.Sprintf("%s@%s", u.Name, u.Host)
}

func (u UserOrRole) SQLString() string {
	if u.Host == "" {
		return fmt.Sprintf("'%s'", u.Name)
	}
	return fmt.Sprintf("'%s'@'%s'", u.Name, u.Host)
}

func (u UserOrRole) Equals(other UserOrRole) bool {
	if u.Name != other.Name {
		return false
	}
	if (u.Host == "" || u.Host == "%") && (other.Host == "" || other.Host == "%") {
		return true
	}
	return u.Host == other.Host
}

type TablePrivilegeGrant struct {
	Database   string
	Table      string
	Privileges []string
	Grant      bool
	UserOrRole UserOrRole
	TLSOption  string
}

func (t *TablePrivilegeGrant) GetId() string {
	return fmt.Sprintf("%s:%s:%s", t.UserOrRole.IDString(), t.GetDatabase(), t.GetTable())
}

func (t *TablePrivilegeGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *TablePrivilegeGrant) GrantOption() bool {
	return t.Grant
}

func (t *TablePrivilegeGrant) GetDatabase() string {
	if t.Database == "*" {
		return "*"
	} else {
		return fmt.Sprintf("`%s`", t.Database)
	}
}

func (t *TablePrivilegeGrant) GetTable() string {
	if t.Table == "*" || t.Table == "" {
		return "*"
	} else {
		return fmt.Sprintf("`%s`", t.Table)
	}
}

func (t *TablePrivilegeGrant) GetPrivileges() []string {
	return t.Privileges
}

func (t *TablePrivilegeGrant) AppendPrivileges(privs []string) {
	t.Privileges = append(t.Privileges, privs...)
}

func (t *TablePrivilegeGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s ON %s.%s TO %s", strings.Join(t.Privileges, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH GRANT OPTION"
	}
	return stmtSql
}

func (t *TablePrivilegeGrant) ConflictsWithGrant(other MySQLGrant) bool {
	otherTyped, ok := other.(*TablePrivilegeGrant)
	if !ok {
		return false
	}
	return otherTyped.GetDatabase() == t.GetDatabase() &&
		otherTyped.GetTable() == t.GetTable()
}

// containsAllPrivilege returns true if the privileges list contains an ALL PRIVILEGES grant
// this is used because there is special case behavior for ALL PRIVILEGES grants. In particular,
// if a user has ALL PRIVILEGES, we _cannot_ revoke ALL PRIVILEGES, GRANT OPTION because this is
// invalid syntax.
// See: https://github.com/petoju/terraform-provider-mysql/issues/120
func containsAllPrivilege(privileges []string) bool {
	for _, p := range privileges {
		if kReAllPrivileges.MatchString(p) {
			return true
		}
	}
	return false
}

func (t *TablePrivilegeGrant) SQLRevokeStatement() string {
	privs := t.Privileges
	if t.Grant && !containsAllPrivilege(privs) {
		privs = append(privs, "GRANT OPTION")
	}
	return fmt.Sprintf("REVOKE %s ON %s.%s FROM %s", strings.Join(privs, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
}

func (t *TablePrivilegeGrant) SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string {
	if t.Grant && !containsAllPrivilege(privilegesToRevoke) {
		privilegesToRevoke = append(privilegesToRevoke, "GRANT OPTION")
	}
	return fmt.Sprintf("REVOKE %s ON %s.%s FROM %s", strings.Join(privilegesToRevoke, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
}

type ProcedurePrivilegeGrant struct {
	Database     string
	ObjectT      ObjectT
	CallableName string
	Privileges   []string
	Grant        bool
	UserOrRole   UserOrRole
	TLSOption    string
}

func (t *ProcedurePrivilegeGrant) GetId() string {
	return fmt.Sprintf("%s:%s:%s", t.UserOrRole.IDString(), t.GetDatabase(), t.GetCallableName())
}

func (t *ProcedurePrivilegeGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *ProcedurePrivilegeGrant) GrantOption() bool {
	return t.Grant
}

func (t *ProcedurePrivilegeGrant) GetDatabase() string {
	if strings.Compare(t.Database, "*") != 0 && !strings.HasSuffix(t.Database, "`") {
		return fmt.Sprintf("`%s`", t.Database)
	}
	return t.Database
}

func (t *ProcedurePrivilegeGrant) GetCallableName() string {
	return fmt.Sprintf("`%s`", t.CallableName)
}

func (t *ProcedurePrivilegeGrant) GetPrivileges() []string {
	return t.Privileges
}

func (t *ProcedurePrivilegeGrant) AppendPrivileges(privs []string) {
	t.Privileges = append(t.Privileges, privs...)
}

func (t *ProcedurePrivilegeGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s ON %s %s.%s TO %s", strings.Join(t.Privileges, ", "), t.ObjectT, t.GetDatabase(), t.GetCallableName(), t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH GRANT OPTION"
	}
	return stmtSql
}

func (t *ProcedurePrivilegeGrant) SQLRevokeStatement() string {
	privs := t.Privileges
	if t.Grant && !containsAllPrivilege(privs) {
		privs = append(privs, "GRANT OPTION")
	}
	stmt := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s", strings.Join(privs, ", "), t.ObjectT, t.GetDatabase(), t.GetCallableName(), t.UserOrRole.SQLString())
	return stmt
}

func (t *ProcedurePrivilegeGrant) SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string {
	privs := privilegesToRevoke
	if t.Grant && !containsAllPrivilege(privilegesToRevoke) {
		privs = append(privs, "GRANT OPTION")
	}
	return fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s", strings.Join(privs, ", "), t.ObjectT, t.GetDatabase(), t.GetCallableName(), t.UserOrRole.SQLString())
}

func (t *ProcedurePrivilegeGrant) ConflictsWithGrant(other MySQLGrant) bool {
	otherTyped, ok := other.(*ProcedurePrivilegeGrant)
	if !ok {
		return false
	}
	return otherTyped.GetDatabase() == t.GetDatabase() &&
		otherTyped.GetCallableName() == t.GetCallableName()
}

type RoleGrant struct {
	Roles      []string
	Grant      bool
	UserOrRole UserOrRole
	TLSOption  string
}

func (t *RoleGrant) GetId() string {
	return fmt.Sprintf("%s", t.UserOrRole.IDString())
}

func (t *RoleGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *RoleGrant) GrantOption() bool {
	return t.Grant
}

func (t *RoleGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT '%s' TO %s", strings.Join(t.Roles, "', '"), t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH ADMIN OPTION"
	}
	return stmtSql
}

func (t *RoleGrant) SQLRevokeStatement() string {
	return fmt.Sprintf("REVOKE '%s' FROM %s", strings.Join(t.Roles, "', '"), t.UserOrRole.SQLString())
}

func (t *RoleGrant) GetRoles() []string {
	return t.Roles
}

func (t *RoleGrant) AppendRoles(roles []string) {
	t.Roles = append(t.Roles, roles...)
}

func (t *RoleGrant) ConflictsWithGrant(other MySQLGrant) bool {
	otherTyped, ok := other.(*RoleGrant)
	if !ok {
		return false
	}
	return otherTyped.GetUserOrRole() == t.GetUserOrRole()
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

func supportsRoles(ctx context.Context, meta interface{}) (bool, error) {
	currentVersion := getVersionFromMeta(ctx, meta)

	requiredVersion, _ := version.NewVersion("8.0.0")
	hasRoles := currentVersion.GreaterThan(requiredVersion)
	return hasRoles, nil
}

var kReProcedureWithoutDatabase = regexp.MustCompile(`(?i)^(function|procedure) ([^.]*)$`)
var kReProcedureWithDatabase = regexp.MustCompile(`(?i)^(function|procedure) ([^.]*)\.([^.]*)$`)

func parseResourceFromData(d *schema.ResourceData) (MySQLGrant, diag.Diagnostics) {

	// Step 1: Parse the user/role
	var userOrRole UserOrRole
	userAttr, userOk := d.GetOk("user")
	hostAttr, hostOk := d.GetOk("host")
	roleAttr, roleOk := d.GetOk("role")
	if (userOk && userAttr.(string) == "") && (roleOk && roleAttr == "") {
		return nil, diag.Errorf("User or role name must be specified")
	}
	if userOk && hostOk && userAttr.(string) != "" && hostAttr.(string) != "" {
		userOrRole = UserOrRole{
			Name: userAttr.(string),
			Host: hostAttr.(string),
		}
	} else if roleOk && roleAttr.(string) != "" {
		userOrRole = UserOrRole{
			Name: roleAttr.(string),
		}
	} else {
		return nil, diag.Errorf("One of user/host or role is required")
	}

	// Step 2: Get generic attributes
	database := d.Get("database").(string)
	tlsOption := d.Get("tls_option").(string)
	grantOption := d.Get("grant").(bool)

	// Step 3a: If `roles` is specified, we have a role grant
	if attr, ok := d.GetOk("roles"); ok {
		roles := setToArray(attr)
		return &RoleGrant{
			Roles:      roles,
			Grant:      grantOption,
			UserOrRole: userOrRole,
			TLSOption:  tlsOption,
		}, nil
	}

	// Step 3b. If the database is a procedure or function, we have a procedure grant
	if kReProcedureWithDatabase.MatchString(database) || kReProcedureWithoutDatabase.MatchString(database) {
		var callableType ObjectT
		var callableName string
		if kReProcedureWithDatabase.MatchString(database) {
			matches := kReProcedureWithDatabase.FindStringSubmatch(database)
			callableType = ObjectT(matches[1])
			database = matches[2]
			callableName = matches[3]
		} else {
			matches := kReProcedureWithoutDatabase.FindStringSubmatch(database)
			callableType = ObjectT(matches[1])
			database = matches[2]
			callableName = d.Get("table").(string)
		}

		privsList := setToArray(d.Get("privileges"))
		privileges := normalizePerms(privsList)

		return &ProcedurePrivilegeGrant{
			Database:     database,
			ObjectT:      callableType,
			CallableName: callableName,
			Privileges:   privileges,
			Grant:        grantOption,
			UserOrRole:   userOrRole,
			TLSOption:    tlsOption,
		}, nil
	}

	// Step 3c. Otherwise, we have a table grant
	privsList := setToArray(d.Get("privileges"))
	privileges := normalizePerms(privsList)

	return &TablePrivilegeGrant{
		Database:   database,
		Table:      d.Get("table").(string),
		Privileges: privileges,
		Grant:      grantOption,
		UserOrRole: userOrRole,
		TLSOption:  tlsOption,
	}, nil
}

func CreateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// Parse the ResourceData
	grant, diagErr := parseResourceFromData(d)
	if err != nil {
		return diagErr
	}

	// Determine whether the database has support for roles
	hasRolesSupport, err := supportsRoles(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}
	if _, ok := grant.(*RoleGrant); ok && !hasRolesSupport {
		return diag.Errorf("role grants are not supported by this version of MySQL")
	}

	// Acquire a lock for the user
	// This is necessary so that the conflicting grant check is correct with respect to other grants being created
	grantCreateMutex.Lock(grant.GetUserOrRole().IDString())
	defer grantCreateMutex.Unlock(grant.GetUserOrRole().IDString())

	// Check to see if there are existing roles that might be clobbered by this grant
	conflictingGrant, err := getMatchingGrant(ctx, db, grant)
	if err != nil {
		return diag.Errorf("failed showing grants: %v", err)
	}
	if conflictingGrant != nil {
		return diag.Errorf("user/role %#v already has grant %v - ", grant.GetUserOrRole(), conflictingGrant)
	}

	stmtSQL := grant.SQLGrantStatement()

	log.Println("[DEBUG] Executing statement:", stmtSQL)
	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("Error running SQL (%v): %v", stmtSQL, err)
	}

	d.SetId(grant.GetId())
	return ReadGrant(ctx, d, meta)
}

func ReadGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting database from Meta: %v", err)
	}

	grantFromTf, diagErr := parseResourceFromData(d)
	if diagErr != nil {
		return diagErr
	}

	grantFromDb, err := getMatchingGrant(ctx, db, grantFromTf)
	if err != nil {
		return diag.Errorf("ReadGrant - getting all grants failed: %v", err)
	}
	if grantFromDb == nil {
		log.Printf("[WARN] GRANT not found for %#v - removing from state", grantFromTf.GetUserOrRole())
		d.SetId("")
		return nil
	}

	setDataFromGrant(grantFromDb, d)

	return nil
}

func UpdateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	if err != nil {
		return diag.Errorf("failed getting user or role: %v", err)
	}

	if d.HasChange("privileges") {
		grant, diagErr := parseResourceFromData(d)
		if diagErr != nil {
			return diagErr
		}

		err = updatePrivileges(ctx, db, d, grant)
		if err != nil {
			return diag.Errorf("failed updating privileges: %v", err)
		}
	}

	return nil
}

func updatePrivileges(ctx context.Context, db *sql.DB, d *schema.ResourceData, grant MySQLGrant) error {
	oldPrivsIf, newPrivsIf := d.GetChange("privileges")
	oldPrivs := oldPrivsIf.(*schema.Set)
	newPrivs := newPrivsIf.(*schema.Set)
	grantIfs := newPrivs.Difference(oldPrivs).List()
	revokeIfs := oldPrivs.Difference(newPrivs).List()

	// Normalize the privileges to revoke
	privsToRevoke := []string{}
	for _, revokeIf := range revokeIfs {
		privsToRevoke = append(privsToRevoke, revokeIf.(string))
	}
	privsToRevoke = normalizePerms(privsToRevoke)

	// Do a partial revoke of anything that has been removed
	if len(privsToRevoke) > 0 {
		partialRevoker, ok := grant.(PrivilegesPartiallyRevocable)
		if !ok {
			return fmt.Errorf("grant does not support partial privilege revokes")
		}
		sqlCommand := partialRevoker.SQLPartialRevokePrivilegesStatement(privsToRevoke)
		log.Printf("[DEBUG] SQL for partial revoke: %s", sqlCommand)

		if _, err := db.ExecContext(ctx, sqlCommand); err != nil {
			return err
		}
	}

	// Do a full grant if anything has been added
	if len(grantIfs) > 0 {
		sqlCommand := grant.SQLGrantStatement()
		log.Printf("[DEBUG] SQL to re-grant privileges: %s", sqlCommand)

		if _, err := db.ExecContext(ctx, sqlCommand); err != nil {
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

	// Parse the grant from ResourceData
	grant, diagErr := parseResourceFromData(d)
	if err != nil {
		return diagErr
	}

	// Acquire a lock for the user
	grantCreateMutex.Lock(grant.GetUserOrRole().IDString())
	defer grantCreateMutex.Unlock(grant.GetUserOrRole().IDString())

	sqlStatement := grant.SQLRevokeStatement()
	log.Printf("[DEBUG] SQL to delete grant: %s", sqlStatement)
	_, err = db.ExecContext(ctx, sqlStatement)
	if err != nil {
		if !isNonExistingGrant(err) {
			return diag.Errorf("error revoking %s: %s", sqlStatement, err)
		}
	}

	return nil
}

func isNonExistingGrant(err error) bool {
	errorNumber := mysqlErrorNumber(err)
	// 1141 = ER_NONEXISTING_GRANT
	// 1147 = ER_NONEXISTING_TABLE_GRANT
	// 1403 = ER_NONEXISTING_PROC_GRANT
	return errorNumber == 1141 || errorNumber == 1147 || errorNumber == 1403
}

func ImportGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	userHostDatabaseTable := strings.Split(d.Id(), "@")

	if len(userHostDatabaseTable) != 4 && len(userHostDatabaseTable) != 5 {
		return nil, fmt.Errorf("wrong ID format %s - expected user@host@database@table (and optionally ending @ to signify grant option) where some parts can be empty)", d.Id())
	}

	user := userHostDatabaseTable[0]
	host := userHostDatabaseTable[1]
	database := userHostDatabaseTable[2]
	table := userHostDatabaseTable[3]
	grantOption := len(userHostDatabaseTable) == 5
	userOrRole := UserOrRole{
		Name: user,
		Host: host,
	}

	desiredGrant := &TablePrivilegeGrant{
		Database:   database,
		Table:      table,
		Grant:      grantOption,
		UserOrRole: userOrRole,
	}

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, fmt.Errorf("got error while getting database from meta: %w", err)
	}

	grants, err := showUserGrants(ctx, db, userOrRole)
	if err != nil {
		return nil, fmt.Errorf("failed to showUserGrants in import: %w", err)
	}
	for _, foundGrant := range grants {
		if foundGrant.ConflictsWithGrant(desiredGrant) {
			res := resourceGrant().Data(nil)
			setDataFromGrant(foundGrant, res)
			return []*schema.ResourceData{res}, nil
		}
	}

	return nil, fmt.Errorf("failed to find the grant to import: %v -- found %#v", userHostDatabaseTable, grants)
}

// setDataFromGrant copies the values from MySQLGrant to the schema.ResourceData
// This function is used when importing a new Grant, or when syncing remote state to Terraform state
// It is responsible for pulling any non-identifying properties (e.g. grant, tls_option) into the Terraform state
// Identifying properties (database, table) are already set either as part of the import id or required properties
// of the Terraform resource.
func setDataFromGrant(grant MySQLGrant, d *schema.ResourceData) *schema.ResourceData {
	if tableGrant, ok := grant.(*TablePrivilegeGrant); ok {
		d.Set("grant", grant.GrantOption())
		d.Set("tls_option", tableGrant.TLSOption)

	} else if procedureGrant, ok := grant.(*ProcedurePrivilegeGrant); ok {
		d.Set("grant", grant.GrantOption())
		d.Set("tls_option", procedureGrant.TLSOption)

	} else if roleGrant, ok := grant.(*RoleGrant); ok {
		d.Set("grant", grant.GrantOption())
		d.Set("roles", roleGrant.Roles)
		d.Set("tls_option", roleGrant.TLSOption)
	} else {
		panic("Unknown grant type")
	}

	// Only set privileges if there is a delta in the normalized privileges
	if grantWithPriv, hasPriv := grant.(MySQLGrantWithPrivileges); hasPriv {
		currentPriv, ok := d.GetOk("privileges")
		if !ok {
			d.Set("privileges", grantWithPriv.GetPrivileges())
		} else {
			currentPrivs := setToArray(currentPriv.(*schema.Set))
			currentPrivs = normalizePerms(currentPrivs)
			if !reflect.DeepEqual(currentPrivs, grantWithPriv.GetPrivileges()) {
				d.Set("privileges", grantWithPriv.GetPrivileges())
			}
		}
	}

	// We need to use the raw pointer to access Table / Database without wrapping them with backticks.
	if tablePrivGrant, isTablePriv := grant.(*TablePrivilegeGrant); isTablePriv {
		d.Set("table", tablePrivGrant.Table)
		d.Set("database", tablePrivGrant.Database)
	}

	// This is a bit of a hack, since we don't have a way to distingush between users and roles
	// from the grant itself. We can only infer it from the schema.
	userOrRole := grant.GetUserOrRole()
	if d.Get("role") != "" {
		d.Set("role", userOrRole.Name)
	} else {
		d.Set("user", userOrRole.Name)
		d.Set("host", userOrRole.Host)
	}

	// This needs to happen for import to work.
	d.SetId(grant.GetId())

	return d
}

func combineGrants(grantA MySQLGrant, grantB MySQLGrant) (MySQLGrant, error) {
	// Check if the grants cover the same user, table, database
	// If not, throw an error because they are unmergeable
	if !grantA.ConflictsWithGrant(grantB) {
		return nil, fmt.Errorf("unable to combine MySQLGrant %s with %s because they don't cover the same table/database/user", grantA, grantB)
	}

	// We can combine grants with privileges
	grantAWithPrivileges, aOk := grantA.(MySQLGrantWithPrivileges)
	grantBWithPrivileges, bOk := grantB.(MySQLGrantWithPrivileges)
	if aOk && bOk {
		grantAWithPrivileges.AppendPrivileges(grantBWithPrivileges.GetPrivileges())
		return grantA, nil
	}

	// We can combine grants with roles
	grantAWithRoles, aOk := grantA.(MySQLGrantWithRoles)
	grantBWithRoles, bOk := grantB.(MySQLGrantWithRoles)
	if aOk && bOk {
		grantAWithRoles.AppendRoles(grantBWithRoles.GetRoles())
		return grantA, nil
	}

	return nil, fmt.Errorf("unable to combine MySQLGrant %s of type %T with %s of type %T", grantA, grantA, grantB, grantB)
}

func getMatchingGrant(ctx context.Context, db *sql.DB, desiredGrant MySQLGrant) (MySQLGrant, error) {
	allGrants, err := showUserGrants(ctx, db, desiredGrant.GetUserOrRole())
	var result MySQLGrant
	if err != nil {
		return nil, fmt.Errorf("showGrant - getting all grants failed: %w", err)
	}
	for _, dbGrant := range allGrants {

		// Check if the grants cover the same user, table, database
		// If not, continue
		if !desiredGrant.ConflictsWithGrant(dbGrant) {
			log.Printf("[DEBUG] Skipping grant %#v as it doesn't match %#v", dbGrant, desiredGrant)
			continue
		}

		// For some reason, MySQL separates privileges into multiple lines
		// So to normalize them, we need to combine them into a single MySQLGrant
		if result != nil {
			result, err = combineGrants(result, dbGrant)
			if err != nil {
				return nil, fmt.Errorf("failed to combine grants in getMatchingGrant: %w", err)
			}
		} else {
			result = dbGrant
		}
	}
	return result, nil
}

var (
	kUserOrRoleRegex = regexp.MustCompile("['`]?([^'`]+)['`]?(?:@['`]?([^'`]+)['`]?)?")
)

func parseUserOrRoleFromRow(userOrRoleStr string) (*UserOrRole, error) {
	userHostMatches := kUserOrRoleRegex.FindStringSubmatch(userOrRoleStr)
	if len(userHostMatches) == 3 {
		return &UserOrRole{
			Name: userHostMatches[1],
			Host: userHostMatches[2],
		}, nil
	} else if len(userHostMatches) == 2 {
		return &UserOrRole{
			Name: userHostMatches[1],
			Host: "%",
		}, nil
	} else {
		return nil, fmt.Errorf("failed to parse user or role portion of grant statement: %s", userOrRoleStr)
	}
}

var (
	kDatabaseAndObjectRegex = regexp.MustCompile("['`]?([^'`]+)['`]?\\.['`]?([^'`]+)['`]?")
)

func parseDatabaseQualifiedObject(objectRef string) (string, string, error) {
	if matches := kDatabaseAndObjectRegex.FindStringSubmatch(objectRef); len(matches) == 3 {
		return matches[1], matches[2], nil
	}
	return "", "", fmt.Errorf("failed to parse database and table portion of grant statement: %s", objectRef)
}

var (
	kRequireRegex = regexp.MustCompile(`.*REQUIRE\s+(.*)`)

	kGrantRegex = regexp.MustCompile(`\bGRANT OPTION\b|\bADMIN OPTION\b`)

	procedureGrantRegex = regexp.MustCompile(`GRANT\s+(.+)\s+ON\s+(FUNCTION|PROCEDURE)\s+(.+)\s+TO\s+(.+)`)
	tableGrantRegex     = regexp.MustCompile(`GRANT\s+(.+)\s+ON\s+(.+)\s+TO\s+(.+)`)
	roleGrantRegex      = regexp.MustCompile(`GRANT\s+(.+)\s+TO\s+(.+)`)
)

func parseGrantFromRow(grantStr string) (MySQLGrant, error) {

	// Ignore REVOKE.*
	if strings.HasPrefix(grantStr, "REVOKE") {
		log.Printf("[WARN] Partial revokes are not fully supported and lead to unexpected behavior. Consult documentation https://dev.mysql.com/doc/refman/8.0/en/partial-revokes.html on how to disable them for safe and reliable terraform. Relevant partial revoke: %s\n", grantStr)
		return nil, nil
	}

	// Parse Require Statement
	tlsOption := "NONE"
	if requireMatches := kRequireRegex.FindStringSubmatch(grantStr); len(requireMatches) == 2 {
		tlsOption = requireMatches[1]
	}

	if procedureMatches := procedureGrantRegex.FindStringSubmatch(grantStr); len(procedureMatches) == 5 {
		privsStr := procedureMatches[1]
		privileges := extractPermTypes(privsStr)
		privileges = normalizePerms(privileges)

		// After normalizePerms, we may have empty privileges. If so, skip this grant.
		if len(privileges) == 0 {
			return nil, nil
		}

		userOrRole, err := parseUserOrRoleFromRow(procedureMatches[4])
		if err != nil {
			return nil, fmt.Errorf("failed to parseUserOrRole for procedure grant: %w", err)
		}

		database, callable, err := parseDatabaseQualifiedObject(procedureMatches[3])
		if err != nil {
			return nil, fmt.Errorf("failed to parseDatabaseQualifiedObject for procedure grant: %w", err)
		}

		grant := &ProcedurePrivilegeGrant{
			Database:     database,
			ObjectT:      ObjectT(procedureMatches[2]),
			CallableName: callable,
			Privileges:   privileges,
			Grant:        kGrantRegex.MatchString(grantStr),
			UserOrRole:   *userOrRole,
			TLSOption:    tlsOption,
		}
		log.Printf("[DEBUG] Got procedure parsed grant: %s, parsed grant is %s: %v", grantStr, reflect.TypeOf(grant), grant)
		return grant, nil
	} else if tableMatches := tableGrantRegex.FindStringSubmatch(grantStr); len(tableMatches) == 4 {
		privsStr := tableMatches[1]
		privileges := extractPermTypes(privsStr)
		privileges = normalizePerms(privileges)

		// After normalizePerms, we may have empty privileges. If so, skip this grant.
		if len(privileges) == 0 {
			return nil, nil
		}

		userOrRole, err := parseUserOrRoleFromRow(tableMatches[3])
		if err != nil {
			return nil, fmt.Errorf("failed to parseUserOrRole for table grant: %w", err)
		}

		database, table, err := parseDatabaseQualifiedObject(tableMatches[2])
		if err != nil {
			return nil, fmt.Errorf("failed to parseDatabaseQualifiedObject for table grant: %w", err)
		}

		grant := &TablePrivilegeGrant{
			Database:   database,
			Table:      table,
			Privileges: privileges,
			Grant:      kGrantRegex.MatchString(grantStr),
			UserOrRole: *userOrRole,
			TLSOption:  tlsOption,
		}
		log.Printf("[DEBUG] Got table parsed grant: %s, parsed grant is %s: %v", grantStr, reflect.TypeOf(grant), grant)
		return grant, nil
	} else if roleMatches := roleGrantRegex.FindStringSubmatch(grantStr); len(roleMatches) == 3 {
		rolesStart := strings.Split(roleMatches[1], ",")
		roles := make([]string, len(rolesStart))

		for i, role := range rolesStart {
			roles[i] = strings.Trim(role, "`@%\" ")
		}

		userOrRole, err := parseUserOrRoleFromRow(roleMatches[2])
		if err != nil {
			return nil, fmt.Errorf("failed to parseUserOrRole for role grant: %w", err)
		}

		grant := &RoleGrant{
			Roles:      roles,
			Grant:      kGrantRegex.MatchString(grantStr),
			UserOrRole: *userOrRole,
			TLSOption:  tlsOption,
		}
		log.Printf("[DEBUG] Got: %s, parsed grant is %s: %v", grantStr, reflect.TypeOf(grant), grant)
		return grant, nil

	} else {
		return nil, fmt.Errorf("failed to parse object portion of grant statement: %s", grantStr)
	}
}

func showUserGrants(ctx context.Context, db *sql.DB, userOrRole UserOrRole) ([]MySQLGrant, error) {
	grants := []MySQLGrant{}

	sqlStatement := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole.SQLString())
	log.Printf("[DEBUG] SQL to show grants: %s", sqlStatement)
	rows, err := db.QueryContext(ctx, sqlStatement)

	if isNonExistingGrant(err) {
		return []MySQLGrant{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("showUserGrants - getting grants failed: %w", err)
	}

	defer rows.Close()
	for rows.Next() {
		var rawGrant string

		err := rows.Scan(&rawGrant)
		if err != nil {
			return nil, fmt.Errorf("showUserGrants - reading row failed: %w", err)
		}

		parsedGrant, err := parseGrantFromRow(rawGrant)
		if err != nil {
			return nil, fmt.Errorf("failed to parseGrantFromRow: %w", err)
		}
		if parsedGrant == nil {
			continue
		}

		// Filter out any grants that don't match the provided user
		// Percona returns also grants for % if we requested IP.
		// Skip them as we don't want terraform to consider it.
		if !parsedGrant.GetUserOrRole().Equals(userOrRole) {
			log.Printf("[DEBUG] Skipping grant for %s as it doesn't match %s", parsedGrant.GetUserOrRole().SQLString(), userOrRole.SQLString())
			continue
		}
		grants = append(grants, parsedGrant)

	}
	log.Printf("[DEBUG] Parsed grants are: %#v", grants)
	return grants, nil
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
	return grants
}

func normalizeColumnOrder(perm string) string {
	re := regexp.MustCompile(`^([^(]*)\((.*)\)$`)
	// We may get inputs like
	// 	SELECT(b,a,c)   -> SELECT(a,b,c)
	// 	DELETE          -> DELETE
	//  SELECT (a,b,c)  -> SELECT(a,b,c)
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
	precursor := strings.Trim(m[1], " ")
	partsTogether := strings.Join(parts, ", ")
	return fmt.Sprintf("%s(%s)", precursor, partsTogether)
}

var kReAllPrivileges = regexp.MustCompile(`\bALL ?(PRIVILEGES)?\b`)

func normalizePerms(perms []string) []string {
	ret := []string{}
	for _, perm := range perms {
		// Remove leading and trailing backticks and spaces
		permNorm := strings.Trim(perm, "` ")
		permUcase := strings.ToUpper(permNorm)

		// Normalize ALL and ALLPRIVILEGES to ALL PRIVILEGES
		if kReAllPrivileges.MatchString(permUcase) {
			permUcase = "ALL PRIVILEGES"
		}
		permSortedColumns := normalizeColumnOrder(permUcase)

		ret = append(ret, permSortedColumns)
	}

	// Remove useless perms
	ret = removeUselessPerms(ret)

	// Sort permissions
	sort.Strings(ret)

	return ret
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
