package mysql

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccProcedure_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// TiDB does not implement stored procedures.
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProcedureConfigBasic,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "id", "tf_test_procedure.greet"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.#", "2"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.mode", "IN"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.name", "person"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.type", "VARCHAR(50)"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.mode", "OUT"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.name", "greeting"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.type", "VARCHAR(100)"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "comment", "says hello"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "deterministic", "true"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "sql_data_access", "NO SQL"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "security_type", "INVOKER"),
					resource.TestCheckResourceAttrSet("mysql_procedure.test", "definer"),
					testAccProcedureCharacteristics("tf_test_procedure", "greet", map[string]string{
						"ROUTINE_COMMENT":  "says hello",
						"IS_DETERMINISTIC": "YES",
						"SQL_DATA_ACCESS":  "NO SQL",
						"SECURITY_TYPE":    "INVOKER",
					}),
				),
			},
			{
				// Characteristics are altered in place, the body is not touched.
				Config: testAccProcedureConfigUpdatedCharacteristics,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "comment", "says hello politely"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "sql_data_access", "CONTAINS SQL"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "security_type", "DEFINER"),
					testAccProcedureCharacteristics("tf_test_procedure", "greet", map[string]string{
						"ROUTINE_COMMENT": "says hello politely",
						"SQL_DATA_ACCESS": "CONTAINS SQL",
						"SECURITY_TYPE":   "DEFINER",
					}),
				),
			},
			{
				// A new body forces the procedure to be recreated.
				Config: testAccProcedureConfigUpdatedBody,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					testAccProcedureCharacteristics("tf_test_procedure", "greet", map[string]string{
						"ROUTINE_DEFINITION": "BEGIN\n  SET greeting = CONCAT('Hi, ', person);\nEND",
					}),
				),
			},
			{
				Config:            testAccProcedureConfigUpdatedBody,
				ResourceName:      "mysql_procedure.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "tf_test_procedure.greet",
			},
		},
	})
}

func TestAccProcedure_noParameters(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProcedureConfigNoParameters,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.#", "0"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "comment", ""),
					resource.TestCheckResourceAttr("mysql_procedure.test", "deterministic", "false"),
					testAccProcedureCharacteristics("tf_test_procedure", "ping", map[string]string{
						"ROUTINE_DEFINITION": "SELECT 1",
						"IS_DETERMINISTIC":   "NO",
						"SQL_DATA_ACCESS":    "CONTAINS SQL",
						"SECURITY_TYPE":      "DEFINER",
					}),
				),
			},
			{
				Config:            testAccProcedureConfigNoParameters,
				ResourceName:      "mysql_procedure.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "tf_test_procedure.ping",
			},
		},
	})
}

func TestAccProcedure_definerAndParameterTypes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProcedureConfigDefinerAndParameterTypes,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "definer", "tf_procedure_definer@localhost"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.#", "2"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.mode", "IN"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.name", "choice"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.type", "ENUM('a','b')"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.mode", "INOUT"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.name", "counter"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.type", "INT"),
					testAccProcedureCharacteristics("tf_test_procedure", "pick", map[string]string{
						"DEFINER": "tf_procedure_definer@localhost",
						// Windows line endings are stored verbatim.
						"ROUTINE_DEFINITION": "BEGIN\r\n  IF choice = 'a' THEN\r\n    SET counter = counter + 1;\r\n  END IF;\r\nEND",
					}),
				),
			},
			{
				Config:            testAccProcedureConfigDefinerAndParameterTypes,
				ResourceName:      "mysql_procedure.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "tf_test_procedure.pick",
			},
		},
	})
}

func testAccProcedureExists(rn string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("procedure id not set")
		}

		count, err := testAccProcedureCount(rs.Primary.ID)
		if err != nil {
			return err
		}

		if count != 1 {
			return fmt.Errorf("expected procedure %s to exist", rs.Primary.ID)
		}

		return nil
	}
}

// testAccProcedureCharacteristics compares columns of information_schema.ROUTINES
// with their expected values.
func testAccProcedureCharacteristics(database, name string, expected map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		for column, want := range expected {
			stmtSQL := fmt.Sprintf(
				"SELECT %s FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'",
				quoteIdentifier(column),
			)
			log.Println("[DEBUG] Executing statement:", stmtSQL)

			var got string
			if err := db.QueryRowContext(ctx, stmtSQL, database, name).Scan(&got); err != nil {
				return fmt.Errorf("error reading %s of procedure %s.%s: %w", column, database, name, err)
			}

			if got != want {
				return fmt.Errorf("expected %s of procedure %s.%s to be %q, got %q", column, database, name, want, got)
			}
		}

		return nil
	}
}

func testAccProcedureCheckDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mysql_procedure" {
			continue
		}

		count, err := testAccProcedureCount(rs.Primary.ID)
		if err != nil {
			return err
		}

		if count > 0 {
			return fmt.Errorf("procedure %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func testAccProcedureCount(id string) (int, error) {
	database, name, err := parseProcedureID(id)
	if err != nil {
		return 0, err
	}

	ctx := context.Background()
	db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return 0, err
	}

	stmtSQL := "SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = ? AND ROUTINE_NAME = ? AND ROUTINE_TYPE = 'PROCEDURE'"
	log.Println("[DEBUG] Executing statement:", stmtSQL)

	var count int
	if err := db.QueryRowContext(ctx, stmtSQL, database, name).Scan(&count); err != nil {
		return 0, fmt.Errorf("error counting procedures: %w", err)
	}

	return count, nil
}

// TestExtractProcedureParameterList covers pulling the parameter list out of a
// CREATE PROCEDURE statement; needs no database.
func TestExtractProcedureParameterList(t *testing.T) {
	testCases := []struct {
		name        string
		createStmt  string
		expected    string
		expectError bool
	}{
		{
			name:       "no parameters",
			createStmt: "CREATE DEFINER=`root`@`localhost` PROCEDURE `ping`()\nSELECT 1",
			expected:   "",
		},
		{
			name:       "parenthesized types",
			createStmt: "CREATE PROCEDURE `p`(IN arg1 DECIMAL(10,2), OUT arg2 ENUM('a','b'))\nBEGIN\nEND",
			expected:   "IN arg1 DECIMAL(10,2), OUT arg2 ENUM('a','b')",
		},
		{
			name:       "parentheses in quoted identifiers",
			createStmt: "CREATE DEFINER=`ro(ot`@`localhost` PROCEDURE `p)1`(IN `a(b` INT)\nBEGIN\nEND",
			expected:   "IN `a(b` INT",
		},
		{
			name:        "no parameter list",
			createStmt:  "CREATE PROCEDURE `p`",
			expectError: true,
		},
		{
			name:        "unterminated quoted identifier",
			createStmt:  "CREATE PROCEDURE `p(IN a INT)",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractProcedureParameterList(tc.createStmt)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// TestParseProcedureParameterList covers parsing declared parameters back into
// the resource schema; needs no database.
func TestParseProcedureParameterList(t *testing.T) {
	testCases := []struct {
		name          string
		parameterList string
		expected      []interface{}
		expectError   bool
	}{
		{
			name:          "empty",
			parameterList: "  ",
			expected:      []interface{}{},
		},
		{
			name:          "implicit IN mode",
			parameterList: "amount DECIMAL(10,2)",
			expected: []interface{}{
				map[string]interface{}{"mode": "IN", "name": "amount", "type": "DECIMAL(10,2)"},
			},
		},
		{
			name:          "every mode",
			parameterList: "IN arg1 INT, OUT arg2 VARCHAR(50), INOUT arg3 BOOL",
			expected: []interface{}{
				map[string]interface{}{"mode": "IN", "name": "arg1", "type": "INT"},
				map[string]interface{}{"mode": "OUT", "name": "arg2", "type": "VARCHAR(50)"},
				map[string]interface{}{"mode": "INOUT", "name": "arg3", "type": "BOOL"},
			},
		},
		{
			name:          "quoted names and commas inside types",
			parameterList: "IN `first name` VARCHAR(50), IN `mode` ENUM('a,b','c')",
			expected: []interface{}{
				map[string]interface{}{"mode": "IN", "name": "first name", "type": "VARCHAR(50)"},
				map[string]interface{}{"mode": "IN", "name": "mode", "type": "ENUM('a,b','c')"},
			},
		},
		{
			name:          "parameter named like a mode",
			parameterList: "in INT, INOUT `out` VARCHAR(10)",
			expected: []interface{}{
				map[string]interface{}{"mode": "IN", "name": "in", "type": "INT"},
				map[string]interface{}{"mode": "INOUT", "name": "out", "type": "VARCHAR(10)"},
			},
		},
		{
			name:          "lowercase mode is normalized",
			parameterList: "inout arg1 INT",
			expected: []interface{}{
				map[string]interface{}{"mode": "INOUT", "name": "arg1", "type": "INT"},
			},
		},
		{
			name:          "missing type",
			parameterList: "IN arg1 INT, arg2",
			expectError:   true,
		},
		{
			name:          "unbalanced parentheses",
			parameterList: "IN arg1 DECIMAL(10,2",
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseProcedureParameterList(tc.parameterList)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

// TestParseProcedureID covers splitting the resource ID; needs no database.
func TestParseProcedureID(t *testing.T) {
	testCases := []struct {
		id               string
		expectedDatabase string
		expectedName     string
		expectError      bool
	}{
		{id: "mydb.myproc", expectedDatabase: "mydb", expectedName: "myproc"},
		{id: "mydb.my.proc", expectedDatabase: "mydb", expectedName: "my.proc"},
		{id: "myproc", expectError: true},
		{id: ".myproc", expectError: true},
		{id: "mydb.", expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			database, name, err := parseProcedureID(tc.id)

			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error, got %q and %q", database, name)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if database != tc.expectedDatabase || name != tc.expectedName {
				t.Fatalf("expected %q and %q, got %q and %q", tc.expectedDatabase, tc.expectedName, database, name)
			}
		})
	}
}

// TestNormalizeDefiner covers definer normalization; needs no database.
func TestNormalizeDefiner(t *testing.T) {
	testCases := []struct {
		definer  string
		expected string
	}{
		{definer: "root@localhost", expected: "root@localhost"},
		{definer: "`root`@`localhost`", expected: "root@localhost"},
		{definer: "'root'@'%'", expected: "root@%"},
		{definer: "jd@oe@localhost", expected: "jd@oe@localhost"},
		{definer: "root", expected: "root"},
		{definer: "", expected: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.definer, func(t *testing.T) {
			if got := normalizeDefiner(tc.definer); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// TestValidateDefiner covers rejecting malformed definers; needs no database.
func TestValidateDefiner(t *testing.T) {
	testCases := []struct {
		definer     string
		expectError bool
	}{
		{definer: "root@localhost"},
		{definer: "`root`@`%`"},
		{definer: "root", expectError: true},
		{definer: "@localhost", expectError: true},
		{definer: "root@", expectError: true},
	}

	for _, tc := range testCases {
		t.Run(tc.definer, func(t *testing.T) {
			_, errs := validateDefiner(tc.definer, "definer")
			if tc.expectError != (len(errs) > 0) {
				t.Fatalf("expected error: %v, got %v", tc.expectError, errs)
			}
		})
	}
}

const testAccProcedureConfigBasic = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "greet"

  parameter {
    name = "person"
    type = "VARCHAR(50)"
  }

  parameter {
    mode = "OUT"
    name = "greeting"
    type = "VARCHAR(100)"
  }

  comment         = "says hello"
  deterministic   = true
  sql_data_access = "NO SQL"
  security_type   = "INVOKER"

  body = <<-SQL
    BEGIN
      SET greeting = CONCAT('Hello, ', person);
    END
  SQL
}
`

const testAccProcedureConfigUpdatedCharacteristics = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "greet"

  parameter {
    name = "person"
    type = "VARCHAR(50)"
  }

  parameter {
    mode = "OUT"
    name = "greeting"
    type = "VARCHAR(100)"
  }

  comment       = "says hello politely"
  deterministic = true

  body = <<-SQL
    BEGIN
      SET greeting = CONCAT('Hello, ', person);
    END
  SQL
}
`

const testAccProcedureConfigUpdatedBody = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "greet"

  parameter {
    name = "person"
    type = "VARCHAR(50)"
  }

  parameter {
    mode = "OUT"
    name = "greeting"
    type = "VARCHAR(100)"
  }

  comment       = "says hello politely"
  deterministic = true

  body = <<-SQL
    BEGIN
      SET greeting = CONCAT('Hi, ', person);
    END
  SQL
}
`

const testAccProcedureConfigDefinerAndParameterTypes = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_user" "definer" {
  user = "tf_procedure_definer"
  host = "localhost"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "pick"
  definer  = "${mysql_user.definer.user}@${mysql_user.definer.host}"

  parameter {
    name = "choice"
    type = "ENUM('a','b')"
  }

  parameter {
    mode = "INOUT"
    name = "counter"
    type = "INT"
  }

  body = "BEGIN\r\n  IF choice = 'a' THEN\r\n    SET counter = counter + 1;\r\n  END IF;\r\nEND"
}
`

const testAccProcedureConfigNoParameters = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "ping"
  body     = "SELECT 1"
}
`
