package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// fakeRolesServer answers the statements mysql_default_roles runs, the way
// MySQL 8 does: DEFAULT ROLE ALL stores every role granted to the user, and
// naming a role that is not granted fails with error 3530.
type fakeRolesServer struct {
	mu       sync.Mutex
	granted  map[string][]string
	defaults map[string][]string
}

var fakeRoles = &fakeRolesServer{granted: map[string][]string{}, defaults: map[string][]string{}}

func init() {
	sql.Register("fake-default-roles", fakeRolesDriver{})
}

type fakeRolesDriver struct{}

func (fakeRolesDriver) Open(string) (driver.Conn, error) { return fakeRolesConn{}, nil }

type fakeRolesConn struct{}

func (fakeRolesConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prepared statements are not supported")
}
func (fakeRolesConn) Close() error { return nil }
func (fakeRolesConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions are not supported")
}

var kFakeAlterDefaultRole = regexp.MustCompile(`^ALTER USER '([^']*)'@'([^']*)' DEFAULT ROLE (.+)$`)

func (fakeRolesConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	m := kFakeAlterDefaultRole.FindStringSubmatch(query)
	if m == nil {
		return nil, fmt.Errorf("unexpected statement: %s", query)
	}
	fakeRoles.mu.Lock()
	defer fakeRoles.mu.Unlock()
	account := m[1] + "@" + m[2]
	switch m[3] {
	case "NONE":
		fakeRoles.defaults[account] = nil
	case "ALL":
		fakeRoles.defaults[account] = append([]string(nil), fakeRoles.granted[account]...)
	default:
		var roles []string
		for _, role := range strings.Split(m[3], ", ") {
			role = strings.Trim(role, "'")
			found := false
			for _, g := range fakeRoles.granted[account] {
				found = found || g == role
			}
			if !found {
				return nil, fmt.Errorf("Error 3530 (HY000): `%s`@`%%` is not granted to `%s`@`%s`", role, m[1], m[2])
			}
			roles = append(roles, role)
		}
		fakeRoles.defaults[account] = roles
	}
	return driver.RowsAffected(0), nil
}

func (fakeRolesConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	fakeRoles.mu.Lock()
	defer fakeRoles.mu.Unlock()
	if query == "SELECT @@GLOBAL.version" {
		return &fakeRolesRows{column: "version", values: []string{"8.0.36"}}, nil
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}
	account := fmt.Sprintf("%v@%v", args[0].Value, args[1].Value)
	switch {
	case strings.Contains(query, "FROM mysql.default_roles"):
		return &fakeRolesRows{column: "default_role_user", values: fakeRoles.defaults[account]}, nil
	case strings.Contains(query, "FROM mysql.role_edges"):
		return &fakeRolesRows{column: "from_user", values: fakeRoles.granted[account]}, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type fakeRolesRows struct {
	column string
	values []string
	next   int
}

func (r *fakeRolesRows) Columns() []string { return []string{r.column} }
func (r *fakeRolesRows) Close() error      { return nil }
func (r *fakeRolesRows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	dest[0] = r.values[r.next]
	r.next++
	return nil
}

func TestDefaultRolesAll(t *testing.T) {
	db, err := sql.Open("fake-default-roles", "")
	if err != nil {
		t.Fatal(err)
	}
	conf := &MySQLConfiguration{Config: &mysql.Config{User: "root", Net: "tcp", Addr: "fake-default-roles:3306"}}
	dsn := conf.Config.FormatDSN()
	connectionCacheMtx.Lock()
	connectionCache[dsn] = &DbConnection{Db: db, Version: version.Must(version.NewVersion("8.0.36"))}
	connectionCacheMtx.Unlock()
	t.Cleanup(func() {
		connectionCacheMtx.Lock()
		delete(connectionCache, dsn)
		connectionCacheMtx.Unlock()
	})

	fakeRoles.mu.Lock()
	fakeRoles.granted["jdoe@%"] = []string{"role1", "role2"}
	fakeRoles.mu.Unlock()

	provider := &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{"mysql_default_roles": resourceDefaultRoles()},
		ConfigureContextFunc: func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
			return conf, nil
		},
	}

	config := func(roles string) string {
		return fmt.Sprintf(`resource "mysql_default_roles" "test" {
	user  = "jdoe"
	host  = "%%"
	roles = %s
}`, roles)
	}
	defaults := func(want ...string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			fakeRoles.mu.Lock()
			defer fakeRoles.mu.Unlock()
			got := append([]string(nil), fakeRoles.defaults["jdoe@%"]...)
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				return fmt.Errorf("default roles are %v, want %v", got, want)
			}
			return nil
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: map[string]func() (*schema.Provider, error){
			"mysql": func() (*schema.Provider, error) { return provider, nil },
		},
		Steps: []resource.TestStep{
			{
				Config: config(`["ALL"]`),
				Check: resource.ComposeTestCheckFunc(
					defaults("role1", "role2"),
					resource.TestCheckResourceAttr("mysql_default_roles.test", "roles.#", "1"),
					resource.TestCheckTypeSetElemAttr("mysql_default_roles.test", "roles.*", "ALL"),
				),
			},
			{
				Config: config(`["role1"]`),
				Check:  defaults("role1"),
			},
			{
				Config: config(`["ALL"]`),
				Check:  defaults("role1", "role2"),
			},
			{
				// A role granted after the apply is not a default role yet.
				PreConfig: func() {
					fakeRoles.mu.Lock()
					fakeRoles.granted["jdoe@%"] = append(fakeRoles.granted["jdoe@%"], "role3")
					fakeRoles.mu.Unlock()
				},
				Config:             config(`["ALL"]`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config(`["ALL"]`),
				Check:  defaults("role1", "role2", "role3"),
			},
			{
				Config: config(`[]`),
				Check:  defaults(),
			},
		},
	})
}
