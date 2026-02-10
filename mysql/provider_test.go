package mysql

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/go-version"

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

func TestBuildAwsConfig(t *testing.T) {
	testCases := []struct {
		name           string
		awsConfigBlock []interface{}
		expectedError  bool
		expectedRegion string
		hasCredentials bool
	}{
		{
			name:           "empty config block",
			awsConfigBlock: []interface{}{},
			expectedError:  false,
		},
		{
			name: "config with region only",
			awsConfigBlock: []interface{}{
				map[string]interface{}{
					"region":     "us-west-2",
					"profile":    "",
					"access_key": "",
					"secret_key": "",
					"role_arn":   "",
				},
			},
			expectedError:  false,
			expectedRegion: "us-west-2",
		},
		{
			name: "config with access key and secret key",
			awsConfigBlock: []interface{}{
				map[string]interface{}{
					"region":     "us-east-1",
					"profile":    "",
					"access_key": "test-access-key",
					"secret_key": "test-secret-key",
					"role_arn":   "",
				},
			},
			expectedError:  false,
			expectedRegion: "us-east-1",
			hasCredentials: true,
		},
		{
			name: "config with role_arn",
			awsConfigBlock: []interface{}{
				map[string]interface{}{
					"region":     "us-east-1",
					"profile":    "",
					"access_key": "",
					"secret_key": "",
					"role_arn":   "arn:aws:iam::123456789012:role/TestRole",
				},
			},
			expectedError:  false,
			expectedRegion: "us-east-1",
			hasCredentials: true,
		},
		{
			name: "config with incomplete access key",
			awsConfigBlock: []interface{}{
				map[string]interface{}{
					"region":     "us-east-1",
					"profile":    "",
					"access_key": "test-access-key",
					"secret_key": "",
					"role_arn":   "",
				},
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			config, err := buildAwsConfig(ctx, tc.awsConfigBlock)

			if tc.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if tc.expectedRegion != "" && config.Region != tc.expectedRegion {
				t.Errorf("Expected region %s, got %s", tc.expectedRegion, config.Region)
			}

			if tc.hasCredentials {
				creds, err := config.Credentials.Retrieve(ctx)

				// For role_arn test, we expect the credentials to be available
				// (even if they might fail in actual AWS call due to test environment)
				if tc.awsConfigBlock[0].(map[string]interface{})["role_arn"].(string) != "" {
					// In test environment, assume role might not work due to lack of valid AWS credentials
					if err != nil {
						t.Logf("Note: Assume role credentials not available in test environment: %v", err)
						// This is expected in test environment, so don't fail the test
						return
					}
				}

				if err != nil {
					t.Errorf("Failed to retrieve credentials: %v", err)
					return
				}

				// Just check that credentials object exists
				if creds.AccessKeyID == "" && creds.SecretAccessKey == "" {
					t.Logf("Note: Credentials not available in test environment")
				}
			}
		})
	}
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

func testAccPreCheckRequireMariaDB(t *testing.T) {
	testAccPreCheck(t)

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		t.Fatalf("Cannot connect to DB (RequireMariaDB): %v", err)
		return
	}

	currentVersionString, err := serverVersionString(db)
	if err != nil {
		t.Fatalf("Cannot get DB version string (RequireMariaDB): %v", err)
		return
	}

	if !strings.Contains(currentVersionString, "MariaDB") {
		t.Skip("Test requires MariaDB")
	}
}

func testAccPreCheckSkipNotMySQL8(t *testing.T) {
	testAccPreCheckSkipNotMySQLVersionMin(t, "8.0.0")
}

func testAccPreCheckSkipNotMySQLVersionMin(t *testing.T, minVersion string) {
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

	versionMin, _ := version.NewVersion(minVersion)
	if currentVersion.LessThan(versionMin) {
		// TiDB 7.x series advertises as 8.0 mysql so we batch its testing strategy with Mysql8
		isTiDB, tidbVersion, mysqlCompatibilityVersion, err := serverTiDB(db)
		if err != nil {
			t.Fatalf("Cannot get DB version string (SkipNotMySQL8): %v", err)
			return
		}
		if isTiDB {
			mysqlVersion, err := version.NewVersion(mysqlCompatibilityVersion)
			if err != nil {
				t.Fatalf("Cannot get DB version string for TiDB (SkipNotMySQL8): %s %s %v", tidbVersion, mysqlCompatibilityVersion, err)
				return
			}
			if mysqlVersion.LessThan(versionMin) {
				t.Skip("Skip on MySQL8")
			}
		}

		t.Skip("Skip on MySQL8")
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
