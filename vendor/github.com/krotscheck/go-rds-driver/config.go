package rds

import (
	"fmt"
	"net/url"
	"strconv"
)

const (
	keyResourceARN = "resource_arn"
	keySecretARN   = "secret_arn"
	keyDatabase    = "database"
	keyAWSRegion   = "aws_region"
	keyParseTime   = "parse_time"
	keySplitMulti  = "split_multi"
)

// Config struct used to provide AWS Configuration Credentials
type Config struct {
	ResourceArn string
	SecretArn   string
	Database    string
	AWSRegion   string
	ParseTime   bool
	SplitMulti  bool
	Custom      map[string][]string
}

// ToDSN converts the config to a DSN string
func (o *Config) ToDSN() string {
	v := url.Values{}
	v.Add(keyResourceARN, o.ResourceArn)
	v.Add(keySecretARN, o.SecretArn)
	v.Add(keyDatabase, o.Database)
	v.Add(keyAWSRegion, o.AWSRegion)
	v.Add(keyParseTime, strconv.FormatBool(o.ParseTime))
	v.Add(keySplitMulti, strconv.FormatBool(o.SplitMulti))

	for k, values := range o.Custom {
		for _, value := range values {
			v.Add(k, value)
		}
	}

	return fmt.Sprintf("%s://?%s", DRIVERNAME, v.Encode())
}

// NewConfigFromDSN assumes that the DSN is a JSON-encoded string
func NewConfigFromDSN(dsn string) (conf *Config, err error) {
	conf = &Config{
		Custom: map[string][]string{},
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// Make sure the scheme is correct.
	if u.Scheme != DRIVERNAME {
		return nil, ErrInvalidDSNScheme
	}

	// All the actual data is in the Query
	values := u.Query()
	for k, vs := range values {
		switch k {
		case keyResourceARN:
			conf.ResourceArn = values.Get(keyResourceARN)
		case keySecretARN:
			conf.SecretArn = values.Get(keySecretARN)
		case keyDatabase:
			conf.Database = values.Get(keyDatabase)
		case keyAWSRegion:
			conf.AWSRegion = values.Get(keyAWSRegion)
		case keyParseTime:
			// Swallow the error here because default is fine.
			parseTime, _ := strconv.ParseBool(values.Get(keyParseTime))
			conf.ParseTime = parseTime
		case keySplitMulti:
			// Swallow the error here because default is fine.
			splitMulti, _ := strconv.ParseBool(values.Get(keySplitMulti))
			conf.SplitMulti = splitMulti
		default:
			// Anything we don't know, store in the custom fields.
			conf.Custom[k] = vs
		}
	}

	return
}

// NewConfig with basic values.
func NewConfig(resourceARN string, secretARN string, database string, awsRegion string) (conf *Config) {
	return &Config{
		ResourceArn: resourceARN,
		SecretArn:   secretARN,
		Database:    database,
		AWSRegion:   awsRegion,
		Custom:      map[string][]string{},
	}
}
