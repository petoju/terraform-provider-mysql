package mysql

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"regexp"
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
				// A new body recreates the procedure in place.
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
					resource.TestCheckResourceAttr("mysql_procedure.test", "definer", "tf_proc_definer@localhost"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.#", "2"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.mode", "IN"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.name", "choice"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.type", "ENUM('a','b')"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.mode", "INOUT"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.name", "counter"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.1.type", "INT"),
					testAccProcedureCharacteristics("tf_test_procedure", "pick", map[string]string{
						"DEFINER": "tf_proc_definer@localhost",
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

func TestAccProcedure_escapedQuotes(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				// ROUTINE_DEFINITION unescapes string literals, so the body must
				// be read back from SHOW CREATE PROCEDURE to avoid a diff.
				Config: testAccProcedureConfigEscapedQuotes,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "body",
						"BEGIN\n  SET message = CONCAT('it''s ', \"a \"\"quoted\"\" \", 'back\\'slash');\nEND"),
				),
			},
			{
				Config:            testAccProcedureConfigEscapedQuotes,
				ResourceName:      "mysql_procedure.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "tf_test_procedure.quote",
			},
		},
	})
}

func TestAccProcedure_recreateKeepsGrants(t *testing.T) {
	grants := map[string]string{
		"tf_proc_exec":  "Execute",
		"tf_proc_admin": "Execute,Alter Routine,Grant",
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccProcedureConfigGrants("", "INT", false, "SELECT 1"),
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					testAccProcedureGrants("tf_test_procedure", "granted", grants),
				),
			},
			{
				// DROP PROCEDURE revokes every privilege on the routine, so the
				// grants must be restored or the plan after this apply would
				// create them again.
				Config: testAccProcedureConfigGrants("", "BIGINT", true, "SELECT 2"),
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureCharacteristics("tf_test_procedure", "granted", map[string]string{
						"ROUTINE_DEFINITION": "SELECT 2",
						"IS_DETERMINISTIC":   "YES",
					}),
					resource.TestCheckResourceAttr("mysql_procedure.test", "parameter.0.type", "BIGINT"),
					testAccProcedureGrants("tf_test_procedure", "granted", grants),
				),
			},
			{
				Config: testAccProcedureConfigGrants("tf_proc_definer@localhost", "BIGINT", true, "SELECT 2"),
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureCharacteristics("tf_test_procedure", "granted", map[string]string{
						"DEFINER": "tf_proc_definer@localhost",
					}),
					testAccProcedureGrants("tf_test_procedure", "granted", grants),
				),
			},
			{
				// The new body is invalid, so the previous definition and its
				// grants must be restored.
				Config:      testAccProcedureConfigGrants("tf_proc_definer@localhost", "BIGINT", true, "SELEC 3"),
				ExpectError: regexp.MustCompile("failed recreating procedure"),
			},
			{
				// A plan only, because an apply would create a missing procedure
				// and its grants again and hide a failed restore.
				Config:   testAccProcedureConfigGrants("tf_proc_definer@localhost", "BIGINT", true, "SELECT 2"),
				PlanOnly: true,
			},
		},
	})
}

func TestAccProcedure_trailingSemicolon(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccPreCheckSkipTiDB(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccProcedureCheckDestroy,
		Steps: []resource.TestStep{
			{
				// The server does not store the final statement terminator, so
				// it must not be kept in state either or every plan would
				// replace the procedure.
				Config: testAccProcedureConfigTrailingSemicolon,
				Check: resource.ComposeTestCheckFunc(
					testAccProcedureExists("mysql_procedure.test"),
					resource.TestCheckResourceAttr("mysql_procedure.test", "body", "BEGIN\n  SELECT 1;\nEND"),
					testAccProcedureCharacteristics("tf_test_procedure", "terminated", map[string]string{
						"ROUTINE_DEFINITION": "BEGIN\n  SELECT 1;\nEND",
					}),
				),
			},
			{
				Config:            testAccProcedureConfigTrailingSemicolon,
				ResourceName:      "mysql_procedure.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "tf_test_procedure.terminated",
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

// testAccProcedureGrants compares the privileges in mysql.procs_priv on a
// procedure with the expected Proc_priv value of each user.
func testAccProcedureGrants(database, name string, expected map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		db, err := connectToMySQL(ctx, testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		rows, err := db.QueryContext(ctx,
			"SELECT User, Proc_priv FROM mysql.procs_priv WHERE Db = ? AND Routine_name = ? AND Routine_type = 'PROCEDURE'",
			database, name,
		)
		if err != nil {
			return fmt.Errorf("error reading the grants on procedure %s.%s: %w", database, name, err)
		}
		defer rows.Close()

		got := map[string]string{}
		for rows.Next() {
			var user, privileges string
			if err := rows.Scan(&user, &privileges); err != nil {
				return err
			}
			got[user] = privileges
		}
		if err := rows.Err(); err != nil {
			return err
		}

		if !reflect.DeepEqual(got, expected) {
			return fmt.Errorf("expected grants on procedure %s.%s to be %v, got %v", database, name, expected, got)
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
			got, _, err := extractProcedureParameterList(tc.createStmt)

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

// TestExtractProcedureBody covers pulling the routine body out of a CREATE
// PROCEDURE statement; needs no database. The statements mirror SHOW CREATE
// PROCEDURE output, which is formatted alike by MySQL 5.6 to 8.4 and MariaDB.
func TestExtractProcedureBody(t *testing.T) {
	testCases := []struct {
		name        string
		createStmt  string
		expected    string
		expectError bool
	}{
		{
			name:       "no characteristics",
			createStmt: "CREATE DEFINER=`root`@`localhost` PROCEDURE `p`()\nSELECT 'x''y'",
			expected:   "SELECT 'x''y'",
		},
		{
			name: "all characteristics",
			createStmt: "CREATE DEFINER=`root`@`localhost` PROCEDURE `p`(IN p_x VARCHAR(10))\n" +
				"    READS SQL DATA\n" +
				"    DETERMINISTIC\n" +
				"    SQL SECURITY INVOKER\n" +
				"    COMMENT 'it''s a (tricky) comment'\n" +
				"BEGIN\n  SET @s = CONCAT('a''b', 'c\\'d', \"e\"\"f\");\nEND",
			expected: "BEGIN\n  SET @s = CONCAT('a''b', 'c\\'d', \"e\"\"f\");\nEND",
		},
		{
			name: "characteristics in any order and case",
			createStmt: "CREATE PROCEDURE `p`() comment \"c\" language sql not deterministic " +
				"no sql sql security definer contains sql modifies sql data SELECT 1",
			expected: "SELECT 1",
		},
		{
			name:       "label named like a characteristic",
			createStmt: "CREATE PROCEDURE `p`()\n    NO SQL\ncomment: BEGIN\nEND",
			expected:   "comment: BEGIN\nEND",
		},
		{
			name:       "windows line endings",
			createStmt: "CREATE PROCEDURE `p`()\r\n    DETERMINISTIC\r\nBEGIN\r\n  SELECT 1;\r\nEND",
			expected:   "BEGIN\r\n  SELECT 1;\r\nEND",
		},
		{
			name:        "no body",
			createStmt:  "CREATE PROCEDURE `p`()\n    DETERMINISTIC",
			expectError: true,
		},
		{
			name:        "unterminated comment",
			createStmt:  "CREATE PROCEDURE `p`() COMMENT 'c",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, remainder, err := extractProcedureParameterList(tc.createStmt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got, err := extractProcedureBody(remainder)

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

// TestProcedureGrantSQL covers turning a mysql.procs_priv row back into a GRANT
// statement; needs no database.
func TestProcedureGrantSQL(t *testing.T) {
	testCases := []struct {
		name        string
		routine     string
		procPriv    string
		expected    string
		expectError bool
	}{
		{
			name:     "execute",
			routine:  "p",
			procPriv: "Execute",
			expected: "GRANT EXECUTE ON PROCEDURE `d`.`p` TO `u`@`%`",
		},
		{
			name:     "all with grant option",
			routine:  "p",
			procPriv: "Execute,Alter Routine,Grant",
			expected: "GRANT EXECUTE, ALTER ROUTINE ON PROCEDURE `d`.`p` TO `u`@`%` WITH GRANT OPTION",
		},
		{
			name:     "grant option only",
			routine:  "p",
			procPriv: "Grant",
			expected: "GRANT USAGE ON PROCEDURE `d`.`p` TO `u`@`%` WITH GRANT OPTION",
		},
		{
			name:     "routine name case is kept",
			routine:  "P",
			procPriv: "Alter Routine",
			expected: "GRANT ALTER ROUTINE ON PROCEDURE `d`.`P` TO `u`@`%`",
		},
		{
			name:        "unknown privilege",
			routine:     "p",
			procPriv:    "Execute,Drop",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			grant, err := newProcedureGrant("d", tc.routine, "u", "%", tc.procPriv)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error, got %q", grant.grantSQL())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := grant.grantSQL(); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// TestTrimBody covers normalizing the body before it is stored; needs no
// database.
func TestTrimBody(t *testing.T) {
	testCases := []struct {
		name     string
		body     string
		expected string
	}{
		{name: "plain", body: "SELECT 1", expected: "SELECT 1"},
		{name: "surrounding whitespace", body: "\n  SELECT 1\n", expected: "SELECT 1"},
		{name: "terminator", body: "SELECT 1;", expected: "SELECT 1"},
		{name: "spaced terminator", body: "SELECT 1 ;\n", expected: "SELECT 1"},
		{name: "repeated terminators", body: "SELECT 1; ;", expected: "SELECT 1"},
		{name: "block with terminator", body: "BEGIN\n  SELECT 1;\nEND;\n", expected: "BEGIN\n  SELECT 1;\nEND"},
		{name: "windows line endings", body: "BEGIN\r\n  SELECT 1;\r\nEND;\r\n", expected: "BEGIN\r\n  SELECT 1;\r\nEND"},
		{name: "terminator inside literal", body: "SELECT ';'", expected: "SELECT ';'"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := trimBody(tc.body); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
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
  user = "tf_proc_definer"
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

const testAccProcedureConfigEscapedQuotes = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "quote"

  parameter {
    mode = "OUT"
    name = "message"
    type = "VARCHAR(100)"
  }

  body = <<-SQL
    BEGIN
      SET message = CONCAT('it''s ', "a ""quoted"" ", 'back\'slash');
    END
  SQL
}
`

const testAccProcedureConfigTrailingSemicolon = `
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_procedure" "test" {
  database = mysql_database.test.name
  name     = "terminated"

  body = <<-SQL
    BEGIN
      SELECT 1;
    END;
  SQL
}
`

// testAccProcedureConfigGrants returns a procedure with two grants on it. An
// empty definer leaves the definer to the server.
func testAccProcedureConfigGrants(definer, parameterType string, deterministic bool, body string) string {
	definerAttribute := ""
	if definer != "" {
		definerAttribute = fmt.Sprintf("definer = %q", definer)
	}

	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "tf_test_procedure"
}

resource "mysql_user" "definer" {
  user = "tf_proc_definer"
  host = "localhost"
}

resource "mysql_user" "exec" {
  user = "tf_proc_exec"
  host = "%%"
}

resource "mysql_user" "admin" {
  user = "tf_proc_admin"
  host = "%%"
}

resource "mysql_procedure" "test" {
  database      = mysql_database.test.name
  name          = "granted"
  deterministic = %t
  body          = %q
  %s

  parameter {
    name = "amount"
    type = %q
  }

  depends_on = [mysql_user.definer]
}

resource "mysql_grant" "exec" {
  user       = mysql_user.exec.user
  host       = mysql_user.exec.host
  database   = "PROCEDURE ${mysql_procedure.test.database}.${mysql_procedure.test.name}"
  privileges = ["EXECUTE"]
}

resource "mysql_grant" "admin" {
  user       = mysql_user.admin.user
  host       = mysql_user.admin.host
  database   = "PROCEDURE ${mysql_procedure.test.database}.${mysql_procedure.test.name}"
  privileges = ["EXECUTE", "ALTER ROUTINE"]
  grant      = true
}
`, deterministic, body, definerAttribute, parameterType)
}

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
