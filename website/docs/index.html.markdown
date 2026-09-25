---
layout: "mysql"
page_title: "Provider: MySQL"
sidebar_current: "docs-mysql-index"
description: |-
  A provider for MySQL Server.
---

# MySQL Provider

[MySQL](http://www.mysql.com) is a relational database server. The MySQL
provider exposes resources used to manage the configuration of resources
in a MySQL server.

Use the navigation to the left to read about the available resources.

## Example Usage

The following is a minimal example:

```hcl
# Configure the MySQL provider
provider "mysql" {
  endpoint = "my-database.example.com:3306"
  username = "app-user"
  password = "app-password"
}

# Create a Database
resource "mysql_database" "app" {
  name = "my_awesome_app"
}
```

This provider can be used in conjunction with other resources that create
MySQL servers. For example, `aws_db_instance` is able to create MySQL
servers in Amazon's RDS service.

```hcl
# Create a database server
resource "aws_db_instance" "default" {
  engine         = "mysql"
  engine_version = "5.6.17"
  instance_class = "db.t1.micro"
  name           = "initial_db"
  username       = "rootuser"
  password       = "rootpasswd"

  # etc, etc; see aws_db_instance docs for more
}

# Configure the MySQL provider based on the outcome of
# creating the aws_db_instance.
provider "mysql" {
  endpoint = "${aws_db_instance.default.endpoint}"
  username = "${aws_db_instance.default.username}"
  password = "${aws_db_instance.default.password}"
}

# Create a second database, in addition to the "initial_db" created
# by the aws_db_instance resource above.
resource "mysql_database" "app" {
  name = "another_db"
}
```

Using encrypted connections can be done by using the `custom_tls` field in the provider

```hcl
provider "mysql" {
  endpoint = "my-database.example.com:3306"
  username = "app-user"
  custom_tls {
    config_key  = "custom_key"
    ca_cert     = "/path/to/certs/ca.pem"
    client_cert = "/path/to/certs/client_cert.pem"
    client_key  = "/path/to/certs/client_key.pem"
  }
}
```

And via Variables:

```hcl
variable "mysql_tls_ca_cert" {
  sensitive = true
  type = string
}
variable "mysql_tls_client_cert" {
  sensitive = true
  type = string
}
variable "mysql_tls_client_key" {
  sensitive = true
  type = string
}

provider "mysql" {
  endpoint = "my-database.example.com:3306"
  username = "app-user"
  custom_tls {
    config_key  = "custom_key"
    ca_cert     = var.mysql_tls_ca_cert
    client_cert = var.mysql_tls_client_cert
    client_key  = var.mysql_tls_client_key
  }
}
```

**Note** It it is _strongly_ recommended to ensure that these values/variables are marked as sensitive

### AWS RDS MySQL server with AWS IAM auth enabled connection

To use this authentication, add `aws://` to the endpoint. This will ignore the `password` field, which will be replaced by an AWS IAM token for the currently obtained identity. You must use `username` and set `tls` to `true` or `skip-verify`, as stated in the AWS documentation.

```hcl
# Configure the MySQL provider for AWS RDS with AWS IAM authentication enabled
provider "mysql" {
  endpoint = "aws://your-rds-instance-name.instance-id.region.rds.amazonaws.com"
  username = "terraform"
  tls      = "skip-verify"
}
```

You can also further configure the AWS connection using the `aws_config` block:

```hcl
# Configure the MySQL provider for AWS RDS with AWS IAM authentication enabled
provider "mysql" {
  endpoint = "aws://your-rds-instance-name.instance-id.region.rds.amazonaws.com"
  username = "terraform"
  tls      = "skip-verify"

  aws_config {
    region      = "your-instance-region"
    profile     = "your-connection-profile"
    access_key  = "your-access-key"
    secret_key  = "your-secret-key"
  }
}
```

See also: [IAM database authentication for MariaDB, MySQL, and PostgreSQL](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html).

### AWS RDS Data API for Aurora

The provider supports AWS RDS Data API for connecting to Aurora databases using the `use_rds_data_api` parameter. This allows you to access Aurora databases without managing VPC configurations or database connections.

#### Prerequisites:

Before using AWS RDS Data API, ensure:

1. **RDS Data API**: You have an Aurora MySQL cluster with Data API enabled
2. **Secrets Manager**: Database credentials are stored in AWS Secrets Manager
3. **IAM Permissions**: Your AWS credentials have permissions to:
   - Execute RDS Data API operations (`rds-data:ExecuteStatement`, `rds-data:BatchExecuteStatement`, etc.)
   - Access the Secrets Manager secret (`secretsmanager:GetSecretValue`)

#### Basic usage with RDS Data API:

```hcl
provider "mysql" {
  # Optional: specify the database in conn_params
  conn_params = {
    database = "mydb"
  }

  aws_config {
    use_rds_data_api = true
    region           = "us-east-1"
    cluster_arn      = "arn:aws:rds:us-east-1:123456789012:cluster:my-aurora-cluster"
    secret_arn       = "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-db-secret"
  }
}
```

#### RDS Data API specific parameters:

- `use_rds_data_api` - Enable RDS Data API mode (mutually exclusive with `aws_rds_iam_auth`)
- `cluster_arn` - ARN of the RDS Aurora cluster (required when `use_rds_data_api = true`)
- `secret_arn` - ARN of the Secrets Manager secret containing database credentials (required when `use_rds_data_api = true`)

#### Important notes:

- RDS Data API is only available for Aurora MySQL clusters
- The `password` parameter must be empty when using RDS Data API
- The `use_rds_data_api` and `aws_rds_iam_auth` options are mutually exclusive
- Connection pool settings (`max_conn_lifetime_sec`, `max_open_conns`, `connect_retry_timeout_sec`) do not apply to RDS Data API as it uses stateless HTTP requests
- Assume role is fully supported - the provider will use the assumed role credentials when accessing both RDS Data API and Secrets Manager
- Some MySQL features may have limitations when using the Data API

### GCP CloudSQL Connection

To connect to a Cloud SQL instance, prefix the endpoint with `cloudsql://` followed by the instance
connection name, in the format `project:region:instance` (or `project/region/instance`). The provider
then reaches the instance through the Cloud SQL Go connector instead of dialing a host directly.

```hcl
provider "mysql" {
  endpoint = "cloudsql://project:region:instance"
  username = "app-user"
  password = "app-password"
}
```

Every connection authenticates twice, and the two are configured independently:

1. **Cloud SQL Admin API**, to obtain the certificate used to reach the instance.
2. **MySQL**, to log in to the database itself.

#### Authenticating to Cloud SQL Admin API

Whichever principal is used here needs `roles/cloudsql.client`.

By default the provider uses the [application default credentials](https://cloud.google.com/docs/authentication/application-default-credentials)
of the environment it runs in: `GOOGLE_APPLICATION_CREDENTIALS`, a `gcloud auth application-default
login` session, or the identity of the VM, GKE pod or Cloud Run service.

Set `impersonate_service_account` in the `gcp_config` block to a service account email to
impersonate it instead. The environment credentials are then only used to mint tokens for that
service account, and those tokens are refreshed automatically. Use this when the identity running Terraform may not connect to the
instance itself:

```hcl
provider "mysql" {
  endpoint = "cloudsql://project:region:instance"

  username = "app-user"
  password = "app-password"

  gcp_config {
    impersonate_service_account = "connector-sa@project-id.iam.gserviceaccount.com"
  }
}
```

This requires `roles/iam.serviceAccountTokenCreator` on the service account for the principal running
Terraform. It only applies to `cloudsql://` endpoints and is ignored elsewhere.

#### Authenticating to MySQL

By default `username` and `password` are an ordinary MySQL user, as in the first example.

Set `iam_database_authentication` in the `gcp_config` block to log in with an IAM principal instead. The identity is
carried by the client certificate, so no password is involved. The instance needs the
`cloudsql_iam_authentication` flag, and the identity needs `roles/cloudsql.instanceUser` and a
database user created for it.

`username` is the identity without the `@` and the domain, so
`sa-name@project-id.iam.gserviceaccount.com` logs in as `sa-name`.

Which identity logs in follows from the Cloud SQL Admin API side:

```hcl
# The impersonated service account
provider "mysql" {
  endpoint = "cloudsql://project:region:instance"

  username = "sa-name"

  gcp_config {
    iam_database_authentication = true
    impersonate_service_account = "sa-name@project-id.iam.gserviceaccount.com"
  }
}
```

```hcl
# The environment's own identity, when it is a database user itself
provider "mysql" {
  endpoint = "cloudsql://project:region:instance"

  username = "runner-sa"

  gcp_config {
    iam_database_authentication = true
  }
}
```

An access token may also be supplied in `password`, in which case it is used for both the Admin API
and the database login, and the identity is the one the token belongs to:

```hcl
provider "mysql" {
  endpoint = "cloudsql://project:region:instance"
  username = "sa-name"
  password = var.access_token # gcloud auth print-access-token

  gcp_config {
    iam_database_authentication = true
  }
}
```

This is the original form of IAM authentication and is kept for compatibility. The token does not
refresh, so runs lasting longer than its lifetime fail part way through, and it must be broadly
scoped because it also serves the Admin API. It remains useful in one case: it is the only setup
that needs no application default credentials at all.

#### Supported combinations

| Cloud SQL Admin API | MySQL | Configuration |
|---|---|---|
| Environment credentials | MySQL user | `username` + `password` |
| Environment credentials | The environment's own identity | `gcp_config.iam_database_authentication` |
| Environment credentials | Owner of a supplied access token | `gcp_config.iam_database_authentication` + token in `password` |
| Impersonated service account | MySQL user | `gcp_config.impersonate_service_account` + `username`/`password` |
| Impersonated service account | The same service account | `gcp_config.impersonate_service_account` + `gcp_config.iam_database_authentication` |

See also:

- [Authentication at Google](https://cloud.google.com/docs/authentication#service-accounts)
- [Roles for service account authentication](https://cloud.google.com/iam/docs/service-account-permissions)
- [Cloud SQL IAM logins](https://cloud.google.com/sql/docs/mysql/iam-logins)

### Azure MySQL server with AzureAD auth enabled connection

To use this authentication, add `azure://` to the endpoint. This will lead to ignore `password` field which would be replaced by Azure AD
token of currently obtained identity. You have to use `username` as stated in Azure documentation.

```hcl
# Configure the MySQL provider for Azure Mysql Server with AzureAD authentication enabled
provider "mysql" {
  endpoint = "azure://your-azure-instance-name.mysql.database.azure.com"
  username = "username@yourtenant.onmicrosoft.com"
  # or if you granted access to AAD group: username = "Active_Directory_GroupName"
}
```

By default the provider will connect using DefaultAzureCredential from the Azure SDK for Go. The credentials can be provided by setting the `AZURE_*` environment variables, using a workload identity or a managed identity present on the host.

You can also further configure the Azure connection using the `azure_config` block:

```hcl
# Configure the MySQL provider for Azure Mysql Server with specific credentials
provider "mysql" {
  endpoint = "azure://your-azure-instance-name.mysql.database.azure.com"
  username = "username@yourtenant.onmicrosoft.com"

  azure_config {
    tenant_id     = "your-tenant-id"
    client_id     = "your-client-id"
    client_secret = var.client_secret
  }
}
```

See also: [Azure Active Directory authentication for MySQL](https://learn.microsoft.com/en-us/azure/mysql/flexible-server/how-to-azure-ad).

## SOCKS5 Proxy Support

The MySQL provider respects the `ALL_PROXY` and/or `all_proxy` environment variables.

```
$ export all_proxy="socks5://your.proxy:3306"
```

## Argument Reference

The following arguments are supported:

- `endpoint` - The address of the MySQL server to use. Most often a "hostname:port" pair, but may also be an absolute path to a Unix socket when the host OS is Unix-compatible. Can also be sourced from the `MYSQL_ENDPOINT` environment variable. This field is optional when `use_rds_data_api` is set to `true` in the `aws_config` block.
- `username` - Username to use to authenticate with the server, can also be sourced from the `MYSQL_USERNAME` environment variable. This field is optional when `use_rds_data_api` is set to `true` in the `aws_config` block.
- `password` - (Optional) Password for the given user, if that user has a password, can also be sourced from the `MYSQL_PASSWORD` environment variable.
- `proxy` - (Optional) Proxy socks url, can also be sourced from `ALL_PROXY` or `all_proxy` environment variables.
- `no_proxy` - (Optional) Comma-separated list of hosts that should not use the proxy. Supports wildcards (`*.example.com`), domain patterns (`.example.com`), CIDR notation (`192.168.0.0/16`), and exact matches. Can also be sourced from `NO_PROXY` or `no_proxy` environment variables.
- `tls` - (Optional) The TLS configuration. One of `false`, `true`, or `skip-verify`. Defaults to `false`. Can also be sourced from the `MYSQL_TLS_CONFIG` environment variable.
- `custom_tls` - (Optional) Sets custom tls options for the connection. Documentation for encrypted connections can be found [here](https://dev.mysql.com/doc/refman/8.0/en/using-encrypted-connections.html). Consider setting shorter `connect_retry_timeout_sec` for debugging, as the default is 5 minutes .This is a block containing an optional `config_key`, which value is discarded but might be useful when troubleshooting, and the following required arguments:

  - `ca_cert` - Local filesystem path or string containing Certificate - If value begins with `-----BEGIN` we assume you're passing the certificate directly, otherwise a file from the local filesystem will be used.
  - `client_cert` - Local filesystem path or string containing Certificate - If value begins with `-----BEGIN` we assume you're passing the certificate directly, otherwise a file from the local filesystem will be used.
  - `client_key` - Local filesystem path or string containing Certificate - If value begins with `-----BEGIN` we assume you're passing the certificate directly, otherwise a file from the local filesystem will be used.

- `max_conn_lifetime_sec` - (Optional) Sets the maximum amount of time a connection may be reused. If d <= 0, connections are reused forever.
- `max_open_conns` - (Optional) Sets the maximum number of open connections to the database. If n <= 0, then there is no limit on the number of open connections.
- `conn_params` - (Optional) Sets extra mysql connection parameters (ODBC parameters). Most useful for session variables such as `default_storage_engine`, `foreign_key_checks` or `sql_log_bin`.
- `authentication_plugin` - (Optional) Sets the authentication plugin, it can be one of the following: `native` or `cleartext`. Defaults to `native`.
- `iam_database_authentication` - (Optional, **deprecated**) Use `gcp_config.iam_database_authentication` instead.
- `private_ip` - (Optional, **deprecated**) Use `gcp_config.private_ip` instead.
- `gcp_config` - (Optional) Sets the GCP configuration for the connection. This argument only applies to CloudSQL and is ignored elsewhere. See [Authenticating to Cloud SQL Admin API](#authenticating-to-cloud-sql-admin-api). This is a block containing the following arguments:
  - `impersonate_service_account` - (Optional) Email of the service account to impersonate when authenticating to Cloud SQL. Can also be sourced from the `MYSQL_GCP_IMPERSONATE_SERVICE_ACCOUNT` environment variable.
  - `iam_database_authentication` - (Optional) Log in to the database with an IAM principal instead of a MySQL password. The identity is the impersonated service account if `impersonate_service_account` is set, the owner of the access token if one is given in `password`, and otherwise the application default credentials of the environment. See [Authenticating to MySQL](#authenticating-to-mysql).
  - `private_ip` - (Optional) Whether to connect to the private IP of the instance. Defaults to `false`.
- `azure_config` - (Optional) Sets the Azure configuration for the connection. This is a block containing the following arguments:
  - `client_id` - (Optional) The client ID for the Azure AD application. Can also be sourced from the `AZURE_CLIENT_ID` or `ARM_CLIENT_ID` environment variables.
  - `client_secret` - (Optional) The client secret for the Azure AD application. Can also be sourced from the `AZURE_CLIENT_SECRET` or `ARM_CLIENT_SECRET` environment variables.
  - `tenant_id` - (Optional) The tenant ID for the Azure AD application. Can also be sourced from the `AZURE_TENANT_ID` or `ARM_TENANT_ID` environment variables.
  - `environment` - (Optional) The Azure environment to use. Can also be sourced from the `AZURE_ENVIRONMENT` or `ARM_ENVIRONMENT` environment variables. Possible values are `public`, `china`, `german`, `usgovernment`. Defaults to `public`.
- `aws_config` - (Optional) Sets the AWS configuration for the connection. This is a block containing the following arguments:
  - `region` - (Optional) AWS Region for the AWS RDS instance. If not provided, it will be sourced by the AWS SDK for Go from standard locations.
  - `profile` - (Optional) AWS SDK configuration profile. If not provided, it will be sourced by the AWS SDK for Go from standard locations.
  - `access_key` - (Optional) AWS Access Key ID. If not provided, it will be sourced by the AWS SDK for Go from standard locations.
  - `secret_key` - (Optional) AWS Secret Access Key. If not provided, it will be sourced by the AWS SDK for Go from standard locations.
  - `role_arn` - (Optional) ARN of the IAM role to assume for RDS authentication.
  - `aws_rds_iam_auth` - (Optional) Enable AWS RDS IAM authentication. When enabled, password is ignored and auth token is generated automatically. Defaults to `false`.
  - `use_rds_data_api` - (Optional) Enable RDS Aurora Data API transport. When enabled, `cluster_arn` and `secret_arn` are required. Defaults to `false`.
  - `cluster_arn` - (Optional) ARN of the RDS Aurora cluster. Required when `use_rds_data_api` is `true`.
  - `secret_arn` - (Optional) ARN of the Secrets Manager secret containing database credentials. Required when `use_rds_data_api` is `true`.
