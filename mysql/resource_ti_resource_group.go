package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type ResourceGroup struct {
	Name          string
	ResourceUnits int
	Priority      string
	Burstable     bool
	QueryLimit    string
}

var CreateResourceGroupSQLPrefix = "CREATE RESOURCE GROUP IF NOT EXISTS"
var UpdateResourceGroupSQLPrefix = "ALTER RESOURCE GROUP"

func (rg *ResourceGroup) buildSQLQuery(prefix string) string {
	var query []string
	baseQuery := fmt.Sprintf("%s %s RU_PER_SEC = %d", prefix, rg.Name, rg.ResourceUnits)
	query = append(query, baseQuery)

	query = append(query, fmt.Sprintf(`PRIORITY = %s`, rg.Priority))

	if rg.QueryLimit == "" {
		query = append(query, fmt.Sprintf(`QUERY_LIMIT=NULL`))
	} else {
		query = append(query, fmt.Sprintf(`QUERY_LIMIT=(%s)`, rg.QueryLimit))
	}

	query = append(query, fmt.Sprintf(`BURSTABLE = %t`, rg.Burstable))
	query = append(query, ";")

	ctx := context.TODO()
	tflog.SetField(ctx, "sql", query)
	tflog.Debug(ctx, `buildSQLQuery`)
	return strings.Join(query, " ")
}

var DefaultResourceGroup = ResourceGroup{
	Name:       "tfDefault",
	Priority:   "medium",
	Burstable:  false,
	QueryLimit: "",
}

var ResourceGroupTiDBMinVersion = "7.5.0"

func resourceTiResourceGroup() *schema.Resource {
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
			// TODO: allow a centralized way to check if there's capacity remaining to use
			"resource_units": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"priority": {
				Type:         schema.TypeString,
				Default:      DefaultResourceGroup.Priority,
				ForceNew:     false,
				ValidateFunc: validation.StringInSlice([]string{"high", "medium", "low"}, false),
				Optional:     true,
			},
			"burstable": {
				Type:     schema.TypeBool,
				Default:  DefaultResourceGroup.Burstable,
				ForceNew: false,
				Optional: true,
			},
			/*
				QUERY_LIMIT=(EXEC_ELAPSED='60s', ACTION=KILL, WATCH=EXACT DURATION='10m')
			*/
			"query_limit": {
				Type:     schema.TypeString,
				Default:  DefaultResourceGroup.QueryLimit,
				ForceNew: false,
				Optional: true,
			},
		},
	}
}

func CreateResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	rg := NewResourceGroupFromResourceData(d)

	var warnLevel, warnMessage string
	var warnCode int = 0

	query := rg.buildSQLQuery(CreateResourceGroupSQLPrefix)
	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "SQL")

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return diag.Errorf("error creating resource group (%s): %s", rg.Name, err)
	}

	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error setting value: %+v Error: %s", rg, warnMessage)
	}

	d.SetId(rg.Name)

	return nil
}

func UpdateResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	rg := NewResourceGroupFromResourceData(d)

	var warnLevel, warnMessage string
	var warnCode int = 0

	query := rg.buildSQLQuery(UpdateResourceGroupSQLPrefix)

	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "SQL")

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return diag.Errorf("error altering resource group (%s): %s", rg.Name, err)
	}

	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error setting value: %s -> %d Error: %s", rg.Name, rg.ResourceUnits, warnMessage)
	}

	d.SetId(rg.Name)

	return nil
}

func ReadResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	rg, err := getResourceGroupFromDB(db, d.Id())
	if err != nil {
		d.SetId("")
		return diag.Errorf("error during get resource group (%s): %s", d.Id(), err)
	}

	// If we're not able to find the resource group, assume that there's terraform
	// diff and allow terraform to recreate it instead of throwing an error.
	if rg == nil {
		d.SetId("")
		return nil
	}

	setResourceGroupOnResourceData(*rg, d)
	return nil
}

func DeleteResourceGroup(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	deleteQuery := fmt.Sprintf("DROP RESOURCE GROUP IF EXISTS %s", name)
	_, err = db.Exec(deleteQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return diag.Errorf("error during drop resource group (%s): %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func getResourceGroupFromDB(db *sql.DB, name string) (*ResourceGroup, error) {
	rg := ResourceGroup{Name: name}

	/*
		Coerce types on SQL side into good types for golang
		Burstable is a varchar(3) so we coerce to BOOLEAN
		QUERY_LIMIT is nullable in DB, but we coerce to standard "empty" string type of ""
		Lowercase priority for less configuration variability
	*/
	query := `SELECT NAME, RU_PER_SEC, LOWER(PRIORITY), BURSTABLE = 'YES' as BURSTABLE, IFNULL(QUERY_LIMIT,"") FROM information_schema.resource_groups WHERE NAME = ?`

	ctx := context.Background()
	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "getResourceGroupFromDB")

	err := db.QueryRow(query, name).Scan(&rg.Name, &rg.ResourceUnits, &rg.Priority, &rg.Burstable, &rg.QueryLimit)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("[DEBUG] resource group doesn't exist (%s): %s", name, err)
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error during get resource group (%s): %s", name, err)
	}

	return &rg, nil
}

func NewResourceGroupFromResourceData(d *schema.ResourceData) ResourceGroup {
	return ResourceGroup{
		Name:          d.Get("name").(string),
		ResourceUnits: d.Get("resource_units").(int),
		Priority:      strings.ToUpper(d.Get("priority").(string)),
		Burstable:     d.Get("burstable").(bool),
		QueryLimit:    d.Get("query_limit").(string),
	}
}

func setResourceGroupOnResourceData(rg ResourceGroup, d *schema.ResourceData) {
	d.Set("name", rg.Name)
	d.Set("resource_units", rg.ResourceUnits)
	d.Set("priority", rg.Priority)
	d.Set("burstable", rg.Burstable)
	d.Set("query_limit", rg.QueryLimit)
}
