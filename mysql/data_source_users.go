package mysql

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUserHash(v interface{}) int {
	user := v.(map[string]interface{})
	return schema.HashString(fmt.Sprintf("%s@%s", user["user"].(string), user["host"].(string)))
}

func dataSourceUsers() *schema.Resource {
	return &schema.Resource{
		ReadContext: ReadUsers,
		Schema: map[string]*schema.Schema{
			"user_pattern": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"host_pattern": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"exclude_users": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"users": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"user": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"host": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func ReadUsers(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	userPattern := d.Get("user_pattern").(string)
	hostPattern := d.Get("host_pattern").(string)

	var excludeUsers []string
	for _, v := range d.Get("exclude_users").([]interface{}) {
		excludeUsers = append(excludeUsers, v.(string))
	}

	sql := fmt.Sprintf("SELECT User,Host FROM mysql.user")

	if userPattern != "" && hostPattern != "" {
		sql += fmt.Sprintf(" WHERE User LIKE '%s' AND Host LIKE '%s'", userPattern, hostPattern)
	} else if userPattern != "" {
		sql += fmt.Sprintf(" WHERE User LIKE '%s'", userPattern)
	} else if hostPattern != "" {
		sql += fmt.Sprintf(" WHERE Host LIKE '%s'", hostPattern)
	}

	log.Printf("[DEBUG] SQL: %s", sql)

	rows, err := db.QueryContext(ctx, sql)
	if err != nil {
		return diag.Errorf("failed querying for users: %v", err)
	}
	defer rows.Close()

	users := schema.NewSet(resourceUserHash, []interface{}{})
	for rows.Next() {
		var user, host string

		if err := rows.Scan(&user, &host); err != nil {
			return diag.Errorf("failed scanning MySQL rows: %v", err)
		}

		key := fmt.Sprintf("%s@%s", user, host)
		if slices.Contains(excludeUsers, key) {
			continue
		}

		item := map[string]interface{}{
			"user": user,
			"host": host,
		}
		users.Add(item)
	}

	if err := d.Set("users", users); err != nil {
		return diag.Errorf("failed setting users: %v", err)
	}

	d.SetId(id.UniqueId())

	return nil
}
