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

func TestProviderAwsRdsIamAuth(t *testing.T) {
	// Temporarily unset MYSQL_PASSWORD to ensure the test is not affected by environment variables.
	// This is necessary because the provider is designed to fail loudly if password is set and nonempty when aws_rds_iam_auth is enabled.
	// By unsetting the environment variable here, we guarantee the test checks only the intended config logic and not external CI state.
	// This does NOT affect normal provider behavior for non-AWS or non-IAM scenarios: if a user sets a password (via config or env) with IAM auth enabled, the provider will still return an error as required.
	orig := os.Getenv("MYSQL_PASSWORD")
	os.Unsetenv("MYSQL_PASSWORD")
	defer os.Setenv("MYSQL_PASSWORD", orig)
	testCases := []struct {
		name          string
		config        map[string]interface{}
		expectedError bool
		errorMessage  string
	}{
		{
			name: "aws_rds_iam_auth enabled with valid aws_config",
			config: map[string]interface{}{
				"endpoint": "test-endpoint.amazonaws.com:3306",
				"username": "test-user",
				"password": "", // Override MYSQL_PASSWORD env var
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"role_arn":         "arn:aws:iam::123456789012:role/TestRole",
						"access_key":       "",
						"secret_key":       "",
						"profile":          "",
						"aws_rds_iam_auth": true,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "aws_rds_iam_auth enabled with password should fail",
			config: map[string]interface{}{
				"endpoint": "test-endpoint.amazonaws.com:3306",
				"username": "test-user",
				"password": "should-not-be-provided",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"role_arn":         "arn:aws:iam::123456789012:role/TestRole",
						"aws_rds_iam_auth": true,
					},
				},
			},
			expectedError: true,
			errorMessage:  "password must be empty when aws_rds_iam_auth is enabled",
		},
		{
			name: "aws_rds_iam_auth disabled should work normally",
			config: map[string]interface{}{
				"endpoint": "test-endpoint.amazonaws.com:3306",
				"username": "test-user",
				"password": "test-password",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"aws_rds_iam_auth": false,
					},
				},
			},
			expectedError: false,
		},
		{
			name: "aws_rds_iam_auth disabled should work normally without aws_config",
			config: map[string]interface{}{
				"endpoint": "test-endpoint.amazonaws.com:3306",
				"username": "test-user",
				"password": "test-password",
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := Provider()
			ctx := context.Background()

			// Create ResourceData from the test config
			raw := make(map[string]interface{})
			for k, v := range tc.config {
				raw[k] = v
			}

			resourceData := schema.TestResourceDataRaw(t, provider.Schema, raw)

			// Test configuration
			_, diags := provider.ConfigureContextFunc(ctx, resourceData)

			if tc.expectedError {
				if !diags.HasError() {
					t.Errorf("Expected error but got none")
					return
				}

				if tc.errorMessage != "" {
					found := false
					for _, diag := range diags {
						if strings.Contains(diag.Summary, tc.errorMessage) || strings.Contains(diag.Detail, tc.errorMessage) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message containing '%s', but got: %v", tc.errorMessage, diags)
					}
				}
			} else {
				if diags.HasError() {
					// For tests that use AWS credentials, we might get auth errors in test environment
					// Check if it's a credential-related error which is expected
					isCredentialError := false
					for _, diag := range diags {
						if strings.Contains(diag.Summary, "failed to build AWS RDS auth token") ||
							strings.Contains(diag.Detail, "InvalidClientTokenId") ||
							strings.Contains(diag.Detail, "NoCredentialProviders") {
							isCredentialError = true
							break
						}
					}

					if !isCredentialError {
						t.Errorf("Unexpected error: %v", diags)
					} else {
						t.Logf("Note: AWS credential error in test environment (expected): %v", diags)
					}
				}
			}
		})
	}
}

func TestProviderRdsDataApiConfig(t *testing.T) {
	// Temporarily unset MYSQL_PASSWORD to ensure the test is not affected by environment variables.
	orig := os.Getenv("MYSQL_PASSWORD")
	os.Unsetenv("MYSQL_PASSWORD")
	defer os.Setenv("MYSQL_PASSWORD", orig)

	testCases := []struct {
		name          string
		config        map[string]interface{}
		expectedError bool
		errorMessage  string
	}{
		{
			name: "use_rds_data_api enabled with valid config",
			config: map[string]interface{}{
				"endpoint": "dummy-endpoint", // Not used with RDS Data API
				"username": "dummy-user",     // Not used with RDS Data API
				"password": "",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"cluster_arn":      "arn:aws:rds:us-east-1:123456789012:cluster:test-cluster",
						"secret_arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret",
					},
				},
			},
			expectedError: false,
		},
		{
			name: "use_rds_data_api enabled without cluster_arn should fail",
			config: map[string]interface{}{
				"endpoint": "dummy-endpoint",
				"username": "dummy-user",
				"password": "",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"cluster_arn":      "",
						"secret_arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret",
					},
				},
			},
			expectedError: true,
			errorMessage:  "cluster_arn is required when use_rds_data_api is true",
		},
		{
			name: "use_rds_data_api enabled without secret_arn should fail",
			config: map[string]interface{}{
				"endpoint": "dummy-endpoint",
				"username": "dummy-user",
				"password": "",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"cluster_arn":      "arn:aws:rds:us-east-1:123456789012:cluster:test-cluster",
						"secret_arn":       "",
					},
				},
			},
			expectedError: true,
			errorMessage:  "secret_arn is required when use_rds_data_api is true",
		},
		{
			name: "use_rds_data_api and aws_rds_iam_auth both enabled should fail",
			config: map[string]interface{}{
				"endpoint": "dummy-endpoint",
				"username": "dummy-user",
				"password": "",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"aws_rds_iam_auth": true,
						"cluster_arn":      "arn:aws:rds:us-east-1:123456789012:cluster:test-cluster",
						"secret_arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret",
					},
				},
			},
			expectedError: true,
			errorMessage:  "use_rds_data_api and aws_rds_iam_auth cannot both be enabled",
		},
		{
			name: "use_rds_data_api with password should fail",
			config: map[string]interface{}{
				"endpoint": "dummy-endpoint",
				"username": "dummy-user",
				"password": "should-not-be-provided",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"cluster_arn":      "arn:aws:rds:us-east-1:123456789012:cluster:test-cluster",
						"secret_arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret",
					},
				},
			},
			expectedError: true,
			errorMessage:  "password must be empty when use_rds_data_api is enabled",
		},
		{
			name: "use_rds_data_api without endpoint and username should succeed",
			config: map[string]interface{}{
				// endpoint and username are omitted
				"password": "",
				"aws_config": []interface{}{
					map[string]interface{}{
						"region":           "us-east-1",
						"use_rds_data_api": true,
						"cluster_arn":      "arn:aws:rds:us-east-1:123456789012:cluster:test-cluster",
						"secret_arn":       "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret",
					},
				},
			},
			expectedError: false,
		},
		{
			name: "without rds_data_api, endpoint is required",
			config: map[string]interface{}{
				"endpoint": "", // Explicitly set to empty to override env var
				"username": "test-user",
				"password": "",
			},
			expectedError: true,
			errorMessage:  "endpoint is required when use_rds_data_api is false",
		},
		{
			name: "without rds_data_api, username is required",
			config: map[string]interface{}{
				"endpoint": "localhost:3306",
				"username": "", // Explicitly set to empty to override env var
				"password": "",
			},
			expectedError: true,
			errorMessage:  "username is required when use_rds_data_api is false",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := Provider()
			ctx := context.Background()

			// Create ResourceData from the test config
			raw := make(map[string]interface{})
			for k, v := range tc.config {
				raw[k] = v
			}

			d := schema.TestResourceDataRaw(t, provider.Schema, raw)

			// Call providerConfigure
			_, diags := providerConfigure(ctx, d)

			if tc.expectedError {
				if !diags.HasError() {
					t.Errorf("Expected error but got none")
				} else {
					// Check if the error message contains the expected text
					found := false
					for _, diag := range diags {
						if strings.Contains(diag.Summary, tc.errorMessage) ||
							strings.Contains(diag.Detail, tc.errorMessage) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error message containing '%s', but got: %v", tc.errorMessage, diags)
					}
				}
			} else {
				if diags.HasError() {
					t.Errorf("Unexpected error: %v", diags)
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
