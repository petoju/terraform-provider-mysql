package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceTiResourceControl() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateResourceGroup,
		ReadContext:   ReadResourceGroup,
		UpdateContext: UpdateResourceGroup,
		DeleteContext: DeleteResourceGroup,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"resource_units": {
				Type:     schema.TypeInt,
				Required: true,
				// TODO: validate
				//ValidateFunc: func(val any, key string) (warns []string, errs []error) {
				//	value := val.(string)
				//	match, _ := regexp.MatchString("(^`(.*)`$|')", value)
				//	if match {
				//		errs = append(errs, fmt.Errorf("%q is badly formatted. %q cant contain any ' string or `<value>`, got: %s", key, key, value))
				//	}
				//	return
				//},
			},
			"priority": {
				Type: schema.TypeString,
				// TODO: ???
				ForceNew:     false,
				ValidateFunc: validation.StringInSlice([]string{"high", "medium", "low"}, true),
				Optional:     true,
			},
			"users": {
				Type: schema.TypeSet,
				// TODO: ???
				ForceNew: false,
				Optional: true,
			},
			//"query_limit": {
			//	Type:     schema.TypeString,
			//	Optional: true,
			//},
		},
	}
}

func CreateResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	varName := d.Get("name").(string)
	varResourceUnits := d.Get("resource_units").(int)
	varPriority := d.Get("priority").(string)
	varBurstable := d.Get("burstable").(bool)
	varUsers := d.Get("users").([]string)

	var warnLevel, warnMessage string
	var warnCode int = 0

	// CREATE RESOURCE GROUP IF NOT EXISTS rg3 RU_PER_SEC = 100 PRIORITY = HIGH BURSTABLE;
	// TODO: requires quoting?
	var query []string
	baseQuery := fmt.Sprintf("CREATE RESOURCE GROUP IF NOT EXISTS %s RU_PER_SEC = %d", varName, varResourceUnits)
	query = append(query, baseQuery)

	if varPriority != "" {
		query = append(query, fmt.Sprintf(`PRIORITY = %s`, varPriority))
	}

	if varBurstable {
		query = append(query, `BURSTABLE`)
	}
	query = append(query, ";")

	log.Printf("[DEBUG] SQL: %s\n", query)

	_, err = db.ExecContext(ctx, strings.Join(query, " "))
	if err != nil {
		return diag.Errorf("error creating resource group (%s): %s", varName, err)
	}

	// TODO: assign RGs to members
	for _, user := range varUsers {
		// ALTER USER usr2 RESOURCE GROUP rg2;
		sql := fmt.Sprintf(`ALTER USER %s RESOURCE GROUP %s;`, user, varName)
		log.Printf("[DEBUG] SQL: %s\n", sql)

		_, err = db.ExecContext(ctx, sql)
		if err != nil {
			return diag.Errorf("error attaching user (%s) to resource group (%s): %s", user, varName, err)
		}
	}

	// TODO: relevant?
	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error setting value: %s -> %d Error: %s", varName, varResourceUnits, warnMessage)
	}

	newId := fmt.Sprintf("%s", varName)

	d.SetId(newId)

	return nil
}

func ReadResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var name, priority string
	var resourceUnits int
	var users []string
	var burstable bool

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// SELECT * FROM information_schema.resource_groups WHERE NAME="default";
	query := fmt.Sprintf(`SELECT NAME, RU_PER_SEC, PRIORITY, BURSTABLE FROM information_schema.resource_groups WHERE NAME="%s";`, d.Id())

	log.Printf("[DEBUG] SQL: %s\n", query)

	err = db.QueryRow(query).Scan(&name, &resourceUnits, &priority, &burstable)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.SetId("")
		return diag.Errorf("error during get resource group (%s): %s", d.Id(), err)
	}

	// Find all users associated with RG
	// SELECT USER, JSON_EXTRACT(User_attributes, "$.resource_group") FROM mysql.user WHERE user = "newuser";

	selectUsersQuery := fmt.Sprintf(`SELECT USER FROM mysql.user WHERE JSON_EXTRACT(User_attributes, "$.resource_group") = %s;`, name)
	rows, err := db.Query(selectUsersQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.SetId("")
		return diag.Errorf("error during get resource group (%s): %s", d.Id(), err)
	}
	for rows.Next() {
		var user string
		err = rows.Scan(&user)
		if err != nil {
			break
		}
		users = append(users, user)
	}

	d.Set("name", name)
	d.Set("resource_units", resourceUnits)
	d.Set("priority", priority)
	d.Set("burstable", burstable)
	d.Set("users", users)

	return nil
}

func DeleteResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)
	users := d.Get("users").([]string)
	if len(users) != 0 {
		return diag.Errorf(`[ERROR]: Cannot delete resource group (%s) when users are assigned`, name)
	}
	log.Printf("[DEBUG]: DELETING RESOURCE GROUP %s\n", name)

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}
	// DROP RESOURCE GROUP IF EXISTS rg1;
	deleteQuery := fmt.Sprintf(`DROP RESOURCE GROUP IF EXISTS %s;`, name)
	_, err = db.Exec(deleteQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return diag.Errorf("error during drop resource group (%s): %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func UpdateResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	varName := d.Get("name").(string)
	varResourceUnits := d.Get("resource_units").(int)
	varPriority := d.Get("priority").(string)
	varBurstable := d.Get("burstable").(bool)
	varUsers := d.Get("users").([]string)

	var warnLevel, warnMessage string
	var warnCode int = 0

	var query []string
	baseQuery := fmt.Sprintf("ALTER RESOURCE GROUP IF EXISTS %s RU_PER_SEC = %d", varName, varResourceUnits)
	query = append(query, baseQuery)

	if varPriority != "" {
		query = append(query, fmt.Sprintf(`PRIORITY = %s`, varPriority))
	}

	if varBurstable {
		query = append(query, `BURSTABLE`)
	}
	query = append(query, ";")

	log.Printf("[DEBUG] SQL: %s\n", query)

	_, err = db.ExecContext(ctx, strings.Join(query, " "))
	if err != nil {
		return diag.Errorf("error altering resource group (%s): %s", varName, err)
	}

	/*
		- Get current users in rg
		- Get desired users in rg
		- Set math to add/remove/etc

	*/
	var usersInDB []string
	selectUsersQuery := fmt.Sprintf(`SELECT USER FROM mysql.user WHERE JSON_EXTRACT(User_attributes, "$.resource_group") = %s;`, varName)
	rows, err := db.Query(selectUsersQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		d.SetId("")
		return diag.Errorf("error during get resource group (%s): %s", d.Id(), err)
	}
	for rows.Next() {
		var user string
		err = rows.Scan(&user)
		if err != nil {
			break
		}
		usersInDB = append(usersInDB, user)
	}

	/*
		- For users in usersInDB, do nothing
		- For usersInDB not in users, set usersInDB member to default RG
	*/
	for _, user := range varUsers {
		if !slices.Contains(usersInDB, user) {
			// Set users to join RG
			sql := fmt.Sprintf(`ALTER USER %s RESOURCE GROUP %s;`, user, varName)
			_, err = db.ExecContext(ctx, sql)
			if err != nil {
				return diag.Errorf("error attaching user (%s) to resource group (%s): %s", user, varName, err)
			}
		}
	}

	for _, dbUser := range usersInDB {
		if !slices.Contains(varUsers, dbUser) {
			// remove from RG
			// Set users to join RG
			sql := fmt.Sprintf("ALTER USER %s RESOURCE GROUP `default`;", dbUser)
			_, err = db.ExecContext(ctx, sql)
			if err != nil {
				return diag.Errorf("error attaching user (%s) to resource group (%s): %s", dbUser, varName, err)
			}
		}

	}

	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error setting value: %s -> %d Error: %s", varName, varResourceUnits, warnMessage)
	}

	newId := varName

	d.SetId(newId)

	return nil
}
