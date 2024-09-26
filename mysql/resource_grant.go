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

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var grantCreateMutex = NewKeyedMutex()

type Grant struct {
	UserIdentity       sql.NullString
	Comment            sql.NullString
	Password           sql.NullString
	Roles              sql.NullString
	GlobalPrivs        sql.NullString
	CatalogPrivs       sql.NullString
	DatabasePrivs      sql.NullString
	TablePrivs         sql.NullString
	ColPrivs           sql.NullString
	ResourcePrivs      sql.NullString
	WorkloadGroupPrivs sql.NullString
}

type EntityType string

const (
	Table         EntityType = "table"
	Resource      EntityType = "resource"
	WorkloadGroup EntityType = "workload_group"
)

func (t EntityType) Equals(other EntityType) bool {
	return t == other
}

type Entity struct {
	Type EntityType
	Name string
}

func (e Entity) IDString() string {
	return fmt.Sprintf("%s:%s", e.Type, e.Name)
}

func (e Entity) SQLString() string {
	switch e.Type {
	case Resource:
		return fmt.Sprintf("RESOURCE '%s'", e.Name)
	case WorkloadGroup:
		return fmt.Sprintf("WORKLOAD GROUP '%s'", e.Name)
	default:
		return e.Name
	}
}

func (e Entity) Equals(other Entity) bool {
	return e.Type == other.Type && e.Name == other.Name
}

// Function to build a list of DorisGrant objects from a Grant object
func buildDorisGrants(grant Grant) ([]DorisGrant, error) {
	var DorisGrants []DorisGrant

	// Helper function to parse a user identity into a name and host
	parseUserIdentity := func(userIdentity string) (string, string) {
		parts := strings.Split(userIdentity, "@")
		// Trim single quotes from each part
		for i := range parts {
			parts[i] = strings.Trim(parts[i], "'")
		}
		if len(parts) == 1 {
			return parts[0], ""
		}
		return parts[0], parts[1]
	}

	// Helper function to build a privilege grant
	buildPrivilegeGrant := func(privs sql.NullString, entityType EntityType) error {
		if privs.Valid && privs.String != "" {
			entries := strings.Split(privs.String, ";")
			for _, entry := range entries {
				var entity Entity = Entity{Type: entityType}
				var privileges string
				entryParts := strings.Split(entry, ":")
				if len(entryParts) == 2 {
					entity.Name = strings.TrimSpace(entryParts[0])
					privileges = strings.TrimSpace(entryParts[1])
				} else if len(entryParts) == 1 {
					// If no target is specified, use global (*.*.*)
					entity.Name = "*.*.*"
					privileges = strings.TrimSpace(entryParts[0])
				} else {
					return fmt.Errorf("invalid privilege format: %s", entry)
				}

				// Ensure that entity.Name matches the format *.*.* for the Table entity type
				if entityType == Table {
					nameParts := strings.Split(strings.TrimSpace(entity.Name), ".")
					for len(nameParts) < 3 {
						nameParts = append(nameParts, "*")
					}
					entity.Name = strings.Join(nameParts, ".")
				}

				name, host := parseUserIdentity(grant.UserIdentity.String)
				DorisGrants = append(DorisGrants, &PrivilegeGrant{
					Privileges: normalizePerms(strings.Split(privileges, ",")),
					Entity:     entity,
					UserOrRole: UserOrRole{
						Name: name,
						Host: host,
					},
				})
			}
		}
		return nil
	}

	// Build GRANT statements for each privilege level
	if err := buildPrivilegeGrant(grant.GlobalPrivs, Table); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.CatalogPrivs, Table); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.DatabasePrivs, Table); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.TablePrivs, Table); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.ColPrivs, Table); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.ResourcePrivs, Resource); err != nil {
		return nil, err
	}
	if err := buildPrivilegeGrant(grant.WorkloadGroupPrivs, WorkloadGroup); err != nil {
		return nil, err
	}

	return DorisGrants, nil
}

type DorisGrant interface {
	GetId() string
	SQLGrantStatement() string
	SQLRevokeStatement() string
	GetUserOrRole() UserOrRole
	ConflictsWithGrant(otherGrant DorisGrant) bool
}

type DorisGrantWithPrivileges interface {
	DorisGrant
	GetPrivileges() []string
	AppendPrivileges([]string)
}

type DorisGrantWithRoles interface {
	DorisGrant
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
		return fmt.Sprintf("ROLE '%s'", u.Name)
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

type PrivilegeGrant struct {
	Privileges []string
	Entity     Entity
	UserOrRole UserOrRole
}

func (t *PrivilegeGrant) GetId() string {
	return fmt.Sprintf("%s:%s", t.UserOrRole.IDString(), t.Entity.IDString())
}

func (t *PrivilegeGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *PrivilegeGrant) GetPrivileges() []string {
	return t.Privileges
}

func (t *PrivilegeGrant) GetEntity() Entity {
	return t.Entity
}

func (t *PrivilegeGrant) AppendPrivileges(privs []string) {
	t.Privileges = append(t.Privileges, privs...)
}

func (t *PrivilegeGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s ON %s TO %s", strings.Join(t.Privileges, ","), t.Entity.SQLString(), t.UserOrRole.SQLString())
	return stmtSql
}

func (t *PrivilegeGrant) ConflictsWithGrant(other DorisGrant) bool {
	otherTyped, ok := other.(*PrivilegeGrant)
	if !ok {
		return false
	}
	return otherTyped.GetEntity() == t.GetEntity()
}

func (t *PrivilegeGrant) SQLRevokeStatement() string {
	privs := t.Privileges
	return fmt.Sprintf("REVOKE %s ON %s FROM %s", strings.Join(privs, ","), t.Entity.SQLString(), t.UserOrRole.SQLString())
}

func (t *PrivilegeGrant) SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string {
	return fmt.Sprintf("REVOKE %s ON %s FROM %s", strings.Join(privilegesToRevoke, ","), t.Entity.SQLString(), t.UserOrRole.SQLString())
}

type RoleGrant struct {
	Roles      []string
	UserOrRole UserOrRole
}

func (t *RoleGrant) GetId() string {
	return t.UserOrRole.IDString()
}

func (t *RoleGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *RoleGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT '%s' TO %s", strings.Join(t.Roles, "','"), t.UserOrRole.SQLString())
	return stmtSql
}

func (t *RoleGrant) SQLRevokeStatement() string {
	return fmt.Sprintf("REVOKE '%s' FROM %s", strings.Join(t.Roles, "','"), t.UserOrRole.SQLString())
}

func (t *RoleGrant) GetRoles() []string {
	return t.Roles
}

func (t *RoleGrant) AppendRoles(roles []string) {
	t.Roles = append(t.Roles, roles...)
}

func (t *RoleGrant) ConflictsWithGrant(other DorisGrant) bool {
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

			"entity_type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(Table),
					string(Resource),
					string(WorkloadGroup),
				}, false),
			},

			"entity_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					if v == "*" {
						errMsg := "Invalid entity name '*'. To match all, use '*.*.*' for tables or '%' for other types."
						errs = append(errs, fmt.Errorf("%s", errMsg))
					}
					return
				},
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
		},
	}
}

func parseResourceFromData(d *schema.ResourceData) (DorisGrant, diag.Diagnostics) {

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

	// Step 2: Get the entity
	entityType := EntityType(d.Get("entity_type").(string))
	entityName := d.Get("entity_name").(string)
	entity := Entity{
		Type: entityType,
		Name: entityName,
	}

	// Step 3a: If `roles` is specified, we have a role grant
	if attr, ok := d.GetOk("roles"); ok {
		roles := setToArray(attr)
		return &RoleGrant{
			Roles:      roles,
			UserOrRole: userOrRole,
		}, nil
	}

	// Step 3b. Otherwise, we have a privilege grant
	privsList := setToArray(d.Get("privileges"))
	privileges := normalizePerms(privsList)

	return &PrivilegeGrant{
		Privileges: privileges,
		Entity:     entity,
		UserOrRole: userOrRole,
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

func updatePrivileges(ctx context.Context, db *sql.DB, d *schema.ResourceData, grant DorisGrant) error {
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
	userHostEntity := strings.Split(d.Id(), "@")

	if len(userHostEntity) != 4 && len(userHostEntity) != 5 {
		return nil, fmt.Errorf("wrong ID format %s - expected user@host@entity (and optionally ending @ to signify grant option) where some parts can be empty)", d.Id())
	}

	user := userHostEntity[0]
	host := userHostEntity[1]
	entityType := userHostEntity[2]
	entityName := userHostEntity[3]
	userOrRole := UserOrRole{
		Name: user,
		Host: host,
	}
	entity := Entity{
		Type: EntityType(entityType),
		Name: entityName,
	}

	desiredGrant := &PrivilegeGrant{
		Entity:     entity,
		UserOrRole: userOrRole,
	}

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, fmt.Errorf("got error while getting database from meta: %w", err)
	}

	grants, err := showPrivilegeGrants(ctx, db, userOrRole)
	if err != nil {
		return nil, fmt.Errorf("failed to showPrivilegeGrants in import: %w", err)
	}
	for _, foundGrant := range grants {
		if foundGrant.ConflictsWithGrant(desiredGrant) {
			res := resourceGrant().Data(nil)
			setDataFromGrant(foundGrant, res)
			return []*schema.ResourceData{res}, nil
		}
	}

	return nil, fmt.Errorf("failed to find the grant to import: %v -- found %#v", userHostEntity, grants)
}

// setDataFromGrant copies the values from DorisGrant to the schema.ResourceData
// This function is used when importing a new Grant, or when syncing remote state to Terraform state
// It is responsible for pulling any non-identifying properties (e.g. grant, tls_option) into the Terraform state
// Identifying properties (database, table) are already set either as part of the import id or required properties
// of the Terraform resource.
func setDataFromGrant(grant DorisGrant, d *schema.ResourceData) *schema.ResourceData {
	if _, ok := grant.(*PrivilegeGrant); ok {
		// Do nothing
	} else if roleGrant, ok := grant.(*RoleGrant); ok {
		d.Set("roles", roleGrant.Roles)
	} else {
		panic("Unknown grant type")
	}

	// Only set privileges if there is a delta in the normalized privileges
	if grantWithPriv, hasPriv := grant.(DorisGrantWithPrivileges); hasPriv {
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

	// We need to use the raw pointer to access Entity without wrapping them with backticks.
	if entityPrivGrant, isEntityPriv := grant.(*PrivilegeGrant); isEntityPriv {
		d.Set("entity_type", entityPrivGrant.Entity.Type)
		d.Set("entity_name", entityPrivGrant.Entity.Name)
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

func getMatchingGrant(ctx context.Context, db *sql.DB, desiredGrant DorisGrant) (DorisGrant, error) {
	allGrants, err := showPrivilegeGrants(ctx, db, desiredGrant.GetUserOrRole())
	if err != nil {
		return nil, fmt.Errorf("showGrant - getting all grants failed: %w", err)
	}
	for _, dbGrant := range allGrants {
		if desiredGrant.ConflictsWithGrant(dbGrant) {
			return dbGrant, nil
		}
		log.Printf("[DEBUG] Skipping grant %#v as it doesn't match %#v", dbGrant, desiredGrant)
	}

	return nil, nil
}

func showPrivilegeGrants(ctx context.Context, db *sql.DB, userOrRole UserOrRole) ([]DorisGrant, error) {
	grants := []DorisGrant{}

	sqlStatement := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole.SQLString())
	log.Printf("[DEBUG] SQL to show grants: %s", sqlStatement)
	rows, err := db.QueryContext(ctx, sqlStatement)

	if isNonExistingGrant(err) {
		return []DorisGrant{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("showPrivilegeGrants - getting grants failed: %w", err)
	}

	defer rows.Close()

	if rows.Next() {
		var grant Grant
		err := rows.Scan(
			&grant.UserIdentity, &grant.Comment, &grant.Password, &grant.Roles, &grant.GlobalPrivs,
			&grant.CatalogPrivs, &grant.DatabasePrivs, &grant.TablePrivs, &grant.ColPrivs,
			&grant.ResourcePrivs, &grant.WorkloadGroupPrivs,
		)
		if err != nil {
			return nil, fmt.Errorf("showPrivilegeGrants - reading row failed: %w", err)
		}
		grants, err = buildDorisGrants(grant)
		if err != nil {
			return nil, fmt.Errorf("failed to buildPrivilegeGrantSQL: %w", err)
		}
	}
	log.Printf("[DEBUG] Parsed grants are: %#v", grants)
	return grants, nil
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

func normalizePerms(perms []string) []string {
	ret := []string{}
	for _, perm := range perms {
		// Remove leading and trailing backticks and spaces
		permNorm := strings.Trim(perm, "` ")
		permUcase := strings.ToUpper(permNorm)

		permSortedColumns := normalizeColumnOrder(permUcase)

		ret = append(ret, permSortedColumns)
	}

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
