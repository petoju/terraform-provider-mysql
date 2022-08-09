package mysql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/creasty/defaults"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/tidwall/gjson"
)

func resourceTiConfigVariable() *schema.Resource {
	return &schema.Resource{
		Create: CreateOrUpdateConfigVariable,
		Read:   ReadConfigVariable,
		Update: CreateOrUpdateConfigVariable,
		Delete: DeleteConfigVariable,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"value": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"pd", "tikv"}, true),
			},
			"instance": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func CreateOrUpdateConfigVariable(d *schema.ResourceData, meta interface{}) error {
	db := meta.(*MySQLConfiguration).Db

	varName := d.Get("name").(string)
	varValue := d.Get("value").(string)
	varInstanceType := d.Get("type").(string)
	varInstance := d.Get("instance").(string)

	var warnLevel, warnMessage string
	var warnCode int = 0

	configQuery := fmt.Sprintf("SET CONFIG %s %s=", varInstanceType, quoteIdentifier(varName))

	if varInstance != "" {
		configQuery = fmt.Sprintf("SET CONFIG \"%s\" %s=", varInstance, quoteIdentifier(varName))
	}

	if isNumeric(varValue) {
		configQuery = fmt.Sprintf("%s %s", configQuery, varValue)
	} else {
		configQuery = fmt.Sprintf("%s'%s'", configQuery, varValue)
	}

	log.Printf("[DEBUG] SQL: %s\n", configQuery)

	_, err := db.Exec(configQuery)
	if err != nil {
		return fmt.Errorf("error setting value: %s", err)
	}

	db.QueryRow("SHOW WARNINGS").Scan(&warnLevel, &warnCode, &warnMessage)

	if warnCode != 0 {
		return fmt.Errorf("error setting value: %s -> %s Error: %s", varName, varValue, warnMessage)
	}

	newId := fmt.Sprintf("%s#%s", varInstanceType, varName)
	if varInstance != "" {
		newId = fmt.Sprintf("%s#%s#%s", varInstanceType, varName, varInstance)
	}

	d.SetId(newId)

	return nil
}

func ReadConfigVariable(d *schema.ResourceData, meta interface{}) error {
	var resType, resInstance, resName, resValue string

	db := meta.(*MySQLConfiguration).Db
	match, _ := regexp.MatchString("^(pd|tikv)#(.*)$", d.Id())
	if !match {
		return fmt.Errorf("error parsing TiDB component (tikv or pd) type from ID.  \n Acceptable format is <pd|tikv>#<config_variable>#<optional_instance>")
	}

	indexParts := strings.Split(d.Id(), "#")
	splitedResType := indexParts[0]
	splitedResName := indexParts[1]

	configQuery := fmt.Sprintf("SHOW CONFIG WHERE type = '%s' AND name = '%s'", splitedResType, splitedResName)
	if len(indexParts) > 2 {
		configQuery = configQuery + fmt.Sprintf(" AND instance = '%s'", (indexParts[2]))
	}

	log.Printf("[DEBUG] SQL: %s\n", configQuery)

	err := db.QueryRow(configQuery).Scan(&resType, &resInstance, &resName, &resValue)
	if err != nil && err != sql.ErrNoRows {
		d.SetId("")
		return fmt.Errorf("error during show config variables: %s", err)
	}

	d.Set("name", resName)
	d.Set("type", resType)
	if len(indexParts) > 2 {
		d.Set("instance", resInstance)
	}
	d.Set("value", resValue)

	return nil

}

func DeleteConfigVariable(d *schema.ResourceData, meta interface{}) error {
	varName := d.Get("name").(string)
	varInstanceType := d.Get("type").(string)
	defCfg := &defaultConfig{}
	var jsonCfg []byte
	var err error

	if err := defaults.Set(defCfg); err != nil {
		return fmt.Errorf("error during destroy config variables: %s", err)
	}

	switch varInstanceType {
	case "pd":
		jsonCfg, err = json.MarshalIndent(&defCfg.Pd, "", "    ")
	case "tikv":
		jsonCfg, err = json.MarshalIndent(&defCfg.TiKv, "", "    ")
	default:
		return fmt.Errorf("error during destory config variables: %s is not allowed type", varInstanceType)
	}

	if err != nil {
		return fmt.Errorf("error during destroy config variables: %s", err)
	}

	log.Printf("[DEBUG] JSON CFG: %s", jsonCfg)
	defaultValue := gjson.Get(string(jsonCfg), varName)
	log.Printf("[DEBUG]: DESTROY %s %s->%s\n", varInstanceType, varName, defaultValue)
	match, _ := regexp.MatchString("^(IGNOREONDESTROY)#(.*)$", defaultValue.String())
	if match {
		log.Printf("[WARN] Variable_name (%s) dont have default values; removing from state", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("value", defaultValue.String())

	return CreateOrUpdateConfigVariable(d, meta)

}
