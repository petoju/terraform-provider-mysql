package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var CreatePlacementPolicySQLPrefix = "CREATE PLACEMENT POLICY IF NOT EXISTS"
var UpdatePlacementPolicySQLPrefix = "ALTER PLACEMENT POLICY"
var BracketsRegex = regexp.MustCompile("^\\[(.+)\\]$")

type PlacementPolicy struct {
	Name          string
	PrimaryRegion string
	Regions       []string
	Constraints   []string
}

func (pp *PlacementPolicy) buildSQLQuery(prefix string) string {
	query := []string{}

	baseQuery := fmt.Sprintf(
		"%s %s",
		prefix,
		pp.Name,
	)

	query = append(query, baseQuery)

	if pp.PrimaryRegion != "" {
		primaryRegionClause := fmt.Sprintf(`PRIMARY_REGION="%s"`, pp.PrimaryRegion)
		query = append(query, primaryRegionClause)
	}

	if len(pp.Regions) > 0 {
		regionsClause := fmt.Sprintf(`REGIONS="%s"`, strings.Join(pp.Regions, ","))
		query = append(query, regionsClause)
	}

	if len(pp.Constraints) > 0 {
		constraintsClause := fmt.Sprintf(`CONSTRAINTS="[%s]"`, strings.Join(pp.Constraints, ","))
		query = append(query, constraintsClause)
	} else {
		// Allow for empty constraints to be set in order to represent a
		// placement policy without constraints.
		query = append(query, `CONSTRAINTS=""`)
	}

	query = append(query, ";")

	ctx := context.Background()
	tflog.SetField(ctx, "sql", query)
	tflog.Debug(ctx, `buildSQLQuery`)

	return strings.Join(query, " ")
}

func resourceTiPlacementPolicy() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreatePlacementPolicy,
		ReadContext:   ReadPlacementPolicy,
		UpdateContext: UpdatePlacementPolicy,
		DeleteContext: DeletePlacementPolicy,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"primary_region": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
				Default:  "",
			},
			"regions": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"constraints": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: false,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func CreatePlacementPolicy(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	pp := NewPlacementPolicyFromResourceData(d)

	var warnLevel, warnMessage string
	var warnCode int = 0

	query := pp.buildSQLQuery(CreatePlacementPolicySQLPrefix)
	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "SQL")

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return diag.Errorf("error creating placement policy (%s): %s", pp.Name, err)
	}

	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error setting value: %+v Error: %s", pp, warnMessage)
	}

	d.SetId(pp.Name)

	return nil
}

func UpdatePlacementPolicy(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	pp := NewPlacementPolicyFromResourceData(d)

	var warnLevel, warnMessage string
	var warnCode int = 0

	query := pp.buildSQLQuery(UpdatePlacementPolicySQLPrefix)

	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "SQL")

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return diag.Errorf("error altering placement policy (%s): %s", pp.Name, err)
	}

	db.QueryRowContext(ctx, "SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)
	if warnCode != 0 {
		return diag.Errorf("error altering placement policy (%s) %s", pp.Name, warnMessage)
	}

	d.SetId(pp.Name)

	return nil
}

func ReadPlacementPolicy(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	pp, err := getPlacementPolicyFromDB(db, d.Id())
	if err != nil {
		d.SetId("")
		return diag.Errorf("error during get placement policy (%s): %s", d.Id(), err)
	}

	// If we're not able to find the placement policy, assume that there's terraform
	// diff and allow terraform to recreate it instead of throwing an error.
	if pp == nil {
		d.SetId("")
		return nil
	}

	setPlacementPolicyOnResourceData(*pp, d)
	return nil
}

func DeletePlacementPolicy(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	name := d.Get("name").(string)

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	deleteQuery := fmt.Sprintf("DROP PLACEMENT POLICY IF EXISTS %s", name)
	_, err = db.Exec(deleteQuery)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return diag.Errorf("error during drop placement policy (%s): %s", d.Id(), err)
	}

	d.SetId("")
	return nil
}

func NewPlacementPolicyFromResourceData(d *schema.ResourceData) PlacementPolicy {
	// Can I cast this directly to []string?
	regionsAny := d.Get("regions").([]any)
	constraintsAny := d.Get("constraints").([]any)
	regions := []string{}
	constraints := []string{}

	for _, regionAny := range regionsAny {
		regions = append(regions, regionAny.(string))
	}

	for _, constraintAny := range constraintsAny {
		constraints = append(constraints, constraintAny.(string))
	}

	return PlacementPolicy{
		Name:          d.Get("name").(string),
		PrimaryRegion: d.Get("primary_region").(string),
		Regions:       regions,
		Constraints:   constraints,
	}
}

func getPlacementPolicyFromDB(db *sql.DB, name string) (*PlacementPolicy, error) {
	pp := PlacementPolicy{Name: name}

	query := `SELECT POLICY_NAME, PRIMARY_REGION, REGIONS, CONSTRAINTS FROM information_schema.placement_policies where POLICY_NAME = ?`

	ctx := context.Background()
	tflog.SetField(ctx, "query", query)
	tflog.Debug(ctx, "getPlacementPolicyFromDB")

	var regionsHolder string
	var constraintsHolder string

	err := db.QueryRow(query, name).Scan(&pp.Name, &pp.PrimaryRegion, &regionsHolder, &constraintsHolder)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("[DEBUG] placement policy doesn't exist (%s): %s", name, err)
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error during get placement policy (%s): %s", name, err)
	}

	if regionsHolder != "" {
		pp.Regions = strings.Split(regionsHolder, ",")
	}

	constraintMatches := BracketsRegex.FindStringSubmatch(constraintsHolder)
	if len(constraintMatches) >= 2 {
		pp.Constraints = strings.Split(constraintMatches[1], ",")
	}

	return &pp, nil
}

func setPlacementPolicyOnResourceData(pp PlacementPolicy, d *schema.ResourceData) {
	d.Set("name", pp.Name)
	d.Set("primary_region", pp.PrimaryRegion)
	d.Set("regions", pp.Regions)
	d.Set("constraints", pp.Constraints)
}
