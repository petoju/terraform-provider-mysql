package mysql

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-version"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// To run these acceptance tests, you will need access to a MySQL server.
// Amazon RDS is one way to get a MySQL server. If you use RDS, you can
// use the root account credentials you specified when creating an RDS
// instance to get the access necessary to run these tests. (the tests
// assume full access to the server.)
//
// Set the MYSQL_ENDPOINT and MYSQL_USERNAME environment variables before
// running the tests. If the given user has a password then you will also need
// to set MYSQL_PASSWORD.
//
// The tests assume a reasonably-vanilla MySQL configuration. In particular,
// they assume that the "utf8" character set is available and that
// "utf8_bin" is a valid collation that isn't the default for that character
// set.
//
// You can run the tests like this:
//    make testacc TEST=./builtin/providers/mysql

var testAccProviderFactories map[string]func() (*schema.Provider, error)

// var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider

func init() {
	testAccProvider = Provider()
	testAccProviderFactories = map[string]func() (*schema.Provider, error){
		"mysql": func() (*schema.Provider, error) { return testAccProvider, nil },
	}
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ = Provider()
}

func testAccPreCheck(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"MYSQL_ENDPOINT", "MYSQL_USERNAME"} {
		if v := os.Getenv(name); v == "" {
			t.Fatal("MYSQL_ENDPOINT, MYSQL_USERNAME and optionally MYSQL_PASSWORD must be set for acceptance tests")
		}
	}

	raw := map[string]interface{}{
		"conn_params": map[string]interface{}{},
	}
	err := testAccProvider.Configure(ctx, terraform.NewResourceConfigRaw(raw))
	if err != nil {
		t.Fatal(err)
	}
}

func testAccPreCheckSkipNotRds(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return
	}

	rdsEnabled, err := serverRds(db)
	if err != nil {
		return
	}

	if !rdsEnabled {
		t.Skip("Skip on non RDS instance")
	}
}

func testAccPreCheckSkipRds(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		if strings.Contains(err.Error(), "SUPER privilege(s) for this operation") {
			t.Skip("Skip on RDS")
		}
		return
	}

	rdsEnabled, err := serverRds(db)
	if err != nil {
		return
	}

	if rdsEnabled {
		t.Skip("Skip on RDS")
	}
}

func testAccPreCheckSkipTiDB(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (SkipTiDB): %v", err)
		return
	}

	currentVersionString, err := serverVersionString(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (SkipTiDB): %v", err)
		return
	}

	if strings.Contains(currentVersionString, "TiDB") {
		t.Skip("Skip on TiDB")
	}
}

func testAccPreCheckSkipMariaDB(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (SkipMariaDB): %v", err)
		return
	}

	currentVersionString, err := serverVersionString(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (SkipMariaDB): %v", err)
		return
	}

	if strings.Contains(currentVersionString, "MariaDB") {
		t.Skip("Skip on MariaDB")
	}
}

func testAccPreCheckSkipNotMySQL8(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (SkipNotMySQL8): %v", err)
		return
	}

	currentVersion, err := serverVersion(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (SkipNotMySQL8): %v", err)
		return
	}

	versionMin, _ := version.NewVersion("8.0.0")
	if currentVersion.LessThan(versionMin) {
		t.Skip("Skip on MySQL8")
	}
}

func testAccPreCheckSkipNotMySQLVersionMin(t *testing.T, minVersion string) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (SkipNotMySQLVersionMin): %v", err)
		return
	}

	currentVersion, err := serverVersion(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (SkipNotMySQLVersionMin): %v", err)
		return
	}

	versionMin, _ := version.NewVersion(minVersion)
	if currentVersion.LessThan(versionMin) {
		t.Skipf("Skip on MySQL version less than %s", minVersion)
	}
}

func testAccPreCheckSkipNotTiDB(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (SkipNotTiDB): %v", err)
		return
	}

	currentVersionString, err := serverVersionString(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (SkipNotTiDB): %v", err)
		return
	}

	if !strings.Contains(currentVersionString, "TiDB") {
		msg := fmt.Sprintf("Skip on MySQL %s", currentVersionString)
		t.Skip(msg)
	}
}
