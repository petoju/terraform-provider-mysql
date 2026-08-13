package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccCloudSQLAuditRule_basic(t *testing.T) {
	cloudSQLAuditRuleUsername := "tf-test-cloud_sql_user"
	cloudSQLAuditRuleDatabase := "tf-test-cloud_sql_db"
	cloudSQLAuditRuleObject := "tf-test-cloud_sql_table"
	cloudSQLAuditRuleOperation := "SELECT"
	cloudSQLAuditRuleOpResult := "S"
	resourceName := "mysql_cloud_sql_audit_rule.test"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheckSkipNotGoogleCloudSQL(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCloudSQLAuditRuleCheckDestroy(cloudSQLAuditRuleUsername),
		Steps: []resource.TestStep{
			{
				Config: testAccCloudSQLAuditRuleConfigBasic(cloudSQLAuditRuleUsername, cloudSQLAuditRuleDatabase, cloudSQLAuditRuleObject, cloudSQLAuditRuleOperation, cloudSQLAuditRuleOpResult),
				Check: resource.ComposeTestCheckFunc(
					testAccCloudSQLAuditRuleExists(cloudSQLAuditRuleUsername),
					resource.TestCheckResourceAttr(resourceName, "username", cloudSQLAuditRuleUsername),
					resource.TestCheckResourceAttr(resourceName, "database", cloudSQLAuditRuleDatabase),
					resource.TestCheckResourceAttr(resourceName, "object", cloudSQLAuditRuleObject),
					resource.TestCheckResourceAttr(resourceName, "operation", cloudSQLAuditRuleOperation),
					resource.TestCheckResourceAttr(resourceName, "op_result", cloudSQLAuditRuleOpResult),
				),
			},
		},
	})
}

func TestCloudSQLAuditRuleCommaSeperatedList(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []interface{}
		wantErr string
	}{
		{
			name:  "multiple database objects",
			input: "users,orders,invoices",
			want:  []interface{}{"users", "orders", "invoices"},
		},
		{
			name:  "schema-qualified database objects",
			input: "public.users, sales.orders, reporting.monthly_invoices",
			want: []interface{}{
				"public.users",
				"sales.orders",
				"reporting.monthly_invoices",
			},
		},
		{
			name:  "database names with underscores and hyphens",
			input: "customer_accounts,order-items,product_catalog",
			want:  []interface{}{"customer_accounts", "order-items", "product_catalog"},
		},
		{
			name:  "trims whitespace around database names",
			input: "  public.users  ,  sales.orders\t, reporting.invoices ",
			want: []interface{}{
				"public.users",
				"sales.orders",
				"reporting.invoices",
			},
		},
		{
			name:  "quoted database name containing a comma",
			input: "public.users,`sales,archive.orders`,reporting.invoices",
			want: []interface{}{
				"public.users",
				"`sales,archive.orders`",
				"reporting.invoices",
			},
		},
		{
			name:  "quoted database name containing an escaped backtick",
			input: "`analytics``archive.events`",
			want:  []interface{}{"`analytics``archive.events`"},
		},
		{
			name:  "escaped backtick in schema-qualified name",
			input: "public.users,`sales``archive.orders,2025`,reporting.invoices",
			want: []interface{}{
				"public.users",
				"`sales``archive.orders,2025`",
				"reporting.invoices",
			},
		},
		{
			name:    "empty list",
			input:   "",
			wantErr: "empty entry in list",
		},
		{
			name:    "empty database name in the middle",
			input:   "public.users,,sales.orders",
			wantErr: "empty entry in list",
		},
		{
			name:    "empty database name at the beginning",
			input:   ",public.users",
			wantErr: "empty entry in list",
		},
		{
			name:    "empty database name at the end",
			input:   "public.users,",
			wantErr: "empty entry in list",
		},
		{
			name:    "whitespace-only database name",
			input:   "public.users,   ,sales.orders",
			wantErr: "empty entry in list",
		},
		{
			name:    "unterminated quoted database name",
			input:   "public.users,`sales.orders",
			wantErr: "unterminated backtick-quoted in list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitList(tt.input)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("splitList(%q) expected an error, got nil", tt.input)
				}

				if err.Error() != tt.wantErr {
					t.Fatalf(
						"splitList(%q) error = %q, want %q",
						tt.input,
						err.Error(),
						tt.wantErr,
					)
				}

				if got != nil {
					t.Fatalf(
						"splitList(%q) result = %#v, want nil",
						tt.input,
						got,
					)
				}

				return
			}

			if err != nil {
				t.Fatalf(
					"splitList(%q) returned unexpected error: %v",
					tt.input,
					err,
				)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf(
					"splitList(%q) = %#v, want %#v",
					tt.input,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestGetCommaSeparatedList(t *testing.T) {
	resourceSchema := map[string]*schema.Schema{
		"database": {
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}

	tests := []struct {
		name  string
		input []interface{}
		want  string
	}{
		{
			name: "multiple database objects",
			input: []interface{}{
				"public.users",
				"sales.orders",
				"reporting.invoices",
			},
			want: "public.users,sales.orders,reporting.invoices",
		},
		{
			name: "schema-qualified database objects",
			input: []interface{}{
				"customer_data.users",
				"billing.transactions",
				"analytics.monthly_revenue",
			},
			want: "customer_data.users,billing.transactions,analytics.monthly_revenue",
		},
		{
			name: "database names with underscores and hyphens",
			input: []interface{}{
				"customer_accounts",
				"order-items",
				"product_catalog",
			},
			want: "customer_accounts,order-items,product_catalog",
		},
		{
			name: "quoted database object names",
			input: []interface{}{
				"`sales,archive.orders`",
				"`analytics``archive.events`",
			},
			want: "`sales,archive.orders`,`analytics``archive.events`",
		},
		{
			name:  "single database object",
			input: []interface{}{"public.users"},
			want:  "public.users",
		},
		{
			name:  "empty list",
			input: []interface{}{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(
				t,
				resourceSchema,
				map[string]interface{}{
					"database": tt.input,
				},
			)

			got := getCommaSeparatedList(d, "database")

			if got != tt.want {
				t.Fatalf(
					"getCommaSeparatedList() = %q, want %q",
					got,
					tt.want,
				)
			}
		})
	}
}

func testAccCloudSQLAuditRuleExists(cloudSQLAuditRuleUsername string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		count, err := testAccGetCloudSQLAuditRuleCount(cloudSQLAuditRuleUsername, db)

		if err != nil {
			return err
		}

		if count > 0 {
			return nil
		}

		return fmt.Errorf("no audit rules found for %s", cloudSQLAuditRuleUsername)
	}
}

func testAccGetCloudSQLAuditRuleCount(cloudSQLAuditRuleUserName string, db *sql.DB) (int, error) {
	query := "CALL mysql.cloudsql_list_audit_rule('*',@outval,@outmsg);"

	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id int
		var username string
		var dbname string
		var object string
		var operation string
		var op_result string
		err := rows.Scan(&id, &username, &dbname, &object, &operation, &op_result)
		if err != nil {
			return 0, fmt.Errorf("error scanning audit rules: %v", err)
		}
		if username == cloudSQLAuditRuleUserName {
			count++

		}
	}

	if rows.Err() != nil {
		return 0, fmt.Errorf("error getting rows: %v", rows.Err())
	}

	return count, nil
}

func testAccCloudSQLAuditRuleCheckDestroy(cloudSQLAuditRuleUsername string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		count, err := testAccGetCloudSQLAuditRuleCount(cloudSQLAuditRuleUsername, db)
		if count > 0 {
			return fmt.Errorf("cloudSQLAuditRule %s still exists", cloudSQLAuditRuleUsername)
		}

		return nil
	}
}

func testAccCloudSQLAuditRuleConfigBasic(cloudSQLAuditRuleUsername string, cloudSQLAuditRuleDatabase string, cloudSQLAuditRuleObject string, cloudSQLAuditRuleOperation string, cloudSQLAuditRuleOpResult string) string {
	return fmt.Sprintf(`
resource "mysql_cloud_sql_audit_rule" "test" {
  username  = ["%s"]
  database  = ["%s"]
  object    = ["%s"]
  operation = ["%s"]
  op_result = "%s"
}
`, cloudSQLAuditRuleUsername, cloudSQLAuditRuleDatabase, cloudSQLAuditRuleObject, cloudSQLAuditRuleOperation, cloudSQLAuditRuleOpResult)
}
