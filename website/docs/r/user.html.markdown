---
layout: "mysql"
page_title: "MySQL: mysql_user"
sidebar_current: "docs-mysql-resource-user"
description: |-
  Creates and manages a user on a MySQL server.
---

# mysql\_user

The ``mysql_user`` resource creates and manages a user on a MySQL
server.

~> **Note:** The password for the user is provided in plain text, and is
obscured by an unsalted hash in the state
[Read more about sensitive data in state](https://www.terraform.io/language/state/sensitive-data).
Care is required when using this resource, to avoid disclosing the password.

## Example Usage

```hcl
resource "mysql_user" "jdoe" {
  user               = "jdoe"
  host               = "example.com"
  plaintext_password = "password"
}
```

## Example Usage with an Authentication Plugin

```hcl
resource "mysql_user" "nologin" {
  user               = "nologin"
  host               = "example.com"
  auth_plugin        = "mysql_no_login"
}
```

## Example Usage with an Authentication Plugin and hashed password

```hcl
resource "mysql_user" "nologin" {
  user               = "nologin"
  host               = "example.com"
  auth_plugin        = "mysql_native_password"
  auth_string_hashed = "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19"
}
```

## Example Usage with caching_sha2_password Authentication Plugin and plaintext password
```hcl
resource "mysql_user" "nologin" {
  user               = "nologin"
  host               = "example.com"
  auth_plugin        = "caching_sha2_password"
  plaintext_password = "password"
}
```
## Example Usage with caching_sha2_password Authentication with Hex Hash

```hcl
resource "mysql_user" "nologin" {
  user            = "nologin"
  host            = "example.com"
  auth_plugin     = "caching_sha2_password"
  auth_string_hex = "0x244124303035246C4F1E0D5D1631594F5C56701F3D327D073A724C706273307A5965516C7756"
}
```

## Example Usage with AzureAD Authentication Plugin

```hcl
resource "mysql_user" "aadupn" {
  user = "aliasToUseWhenConnectiong"
  auth_plugin = "aad_auth"
  aad_identity {
    type = "user" # user | group | service_principal
    identity = "little.johny@doe.onmicrosoft.com" # upn | group name | client id of service principal
  }
}
```

~> **Note on Azure Database for MySQL Single Server resource:** If you want to use this for `service_principal` with older Azure Database for MySQL Single Server resource, you need to set param `aad_auth_validate_oids_in_tenant` to `OFF` in provider configuration. For more details see [this issue](https://github.com/petoju/terraform-provider-mysql/issues/79).

## Example Usage with Resource Limits

```hcl
# MySQL and MariaDB: Set MAX_USER_CONNECTIONS
resource "mysql_user" "limited" {
  user                 = "app_user"
  host                 = "%"
  plaintext_password   = "password"
  max_user_connections = 100
}

# MariaDB only: Set both MAX_USER_CONNECTIONS and MAX_STATEMENT_TIME
resource "mysql_user" "limited_mariadb" {
  user                 = "app_user"
  host                 = "%"
  plaintext_password   = "password"
  max_user_connections = 100
  max_statement_time   = 30.0  # 30 seconds
}

# MariaDB only: Fractional values for subsecond precision
resource "mysql_user" "limited_precise" {
  user                 = "app_user"
  host                 = "%"
  plaintext_password   = "password"
  max_statement_time   = 0.5  # 500 milliseconds
}
```

## Argument Reference

The following arguments are supported:

* `user` - (Required) The name of the user.
* `host` - (Optional) The source host of the user. Defaults to "localhost".
* `plaintext_password` - (Optional) The password for the user. This must be provided in plain text, so the data source for it must be secured. An _unsalted_ hash of the provided password is stored in state.
* `password` - (Optional) Deprecated alias of `plaintext_password`, whose value is _stored as plaintext in state_. Prefer to use `plaintext_password` instead, which stores the password as an unsalted hash.
* `password_wo` - (Optional) The write-only plaintext password that accepts plain text like `plaintext_password` but is not stored in state. Cannot be used with `plaintext_password`, `password`, `auth_string_hashed`, or `auth_string_hex`.
* `password_wo_version` - (Optional) Used together with `password_wo` to trigger password changes. Whenever the version is changed, the password provided in `password_wo` is applied to the user.
* `auth_plugin` - (Optional) Use an [authentication plugin][ref-auth-plugins] to authenticate the user instead of using password authentication.  Description of the fields allowed in the block below.
* `auth_string_hashed` - (Optional) Use an already hashed string as a parameter to `auth_plugin`. This can be used with passwords as well as with other auth strings.
* `auth_string_hex` - (Optional) The authentication string as a hexadecimal value(can be with or without `0x` prefix). Primarily used with `caching_sha2_password` authentication plugin. Cannot be used with `plaintext_password`, `password`, `password_wo`, or `auth_string_hashed`.
* `aad_identity` - (Optional) Required when `auth_plugin` is `aad_auth`. This should be block containing `type` and `identity`. `type` can be one of `user`, `group` and `service_principal`. `identity` then should containt either UPN of user, name of group or Client ID of service principal.
* `retain_old_password` - (Optional) When `true`, the old password is retained when changing the password. Defaults to `false`. This use MySQL Dual Password Support feature and requires MySQL version 8.0.14 or newer. See [MySQL Dual Password documentation](https://dev.mysql.com/doc/refman/8.0/en/password-management.html#dual-passwords) for more.
* `discard_old_password` - (Optional) When `true`, the old password is deleted. Defaults to `false`. This use MySQL Dual Password Support feature and requires MySQL version 8.0.14 or newer. See [MySQL Dual Password documentation](https://dev.mysql.com/doc/refman/8.0/en/password-management.html#dual-passwords) for more.
* `tls_option` - (Optional) An TLS-Option for the `CREATE USER` or `ALTER USER` statement. The value is suffixed to `REQUIRE`. A value of 'SSL' will generate a `CREATE USER ... REQUIRE SSL` statement. See the [MYSQL `CREATE USER` documentation](https://dev.mysql.com/doc/refman/5.7/en/create-user.html) for more. Ignored if MySQL version is under 5.7.0.
* `max_user_connections` - (Optional) Maximum number of simultaneous connections the user can have. A value of `0` (the default) means unlimited. Supported on MySQL 5.0+ and all MariaDB versions. When this argument is removed from the configuration, the limit is reset to `0` (unlimited).
* `max_statement_time` - (Optional) Maximum execution time for statements in seconds. A value of `0` (the default) means unlimited. Supports fractional values for subsecond precision (e.g., `0.01` for 10 milliseconds, `30.5` for 30.5 seconds). **Only supported on MariaDB 10.1.1 or newer.** Attempting to use this on MySQL will result in an error. When this argument is removed from the configuration, the limit is reset to `0` (unlimited).

[ref-auth-plugins]: https://dev.mysql.com/doc/refman/5.7/en/authentication-plugins.html

The `auth_plugin` value supports:

* `AWSAuthenticationPlugin` - Allows the use of IAM authentication with [Amazon
  Aurora][ref-amazon-aurora]. For more details on how to use IAM auth with
  Aurora, see [here][ref-aurora-using-iam].

[ref-amazon-aurora]: https://aws.amazon.com/rds/aurora/
[ref-aurora-using-iam]: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html#UsingWithRDS.IAMDBAuth.Creating

* `mysql_no_login` - Uses the MySQL No-Login Authentication Plugin. The
  No-Login Authentication Plugin must be active in MySQL. For more information,
  see [here][ref-mysql-no-login].

[ref-mysql-no-login]: https://dev.mysql.com/doc/refman/5.7/en/no-login-pluggable-authentication.html

* `aad_auth` - Uses `CREATE AADUSER` statement to create user instead of `CREATE USER` to create user
   with [AzureAD authentication][ref-azure-aadauth] to [Azure Database for MySQL][ref-azure-mysql].
   When specified, you need to specify `aad_identity`. For more information about AzureAD authentication into MySQL  
   see [here][ref-azure-aadauth]. You have to use AAD authenticated administrator mysql session to use this plugin.

[ref-azure-aadauth]: https://learn.microsoft.com/en-us/azure/mysql/flexible-server/how-to-azure-ad
[ref-azure-mysql]: https://learn.microsoft.com/en-us/azure/mysql/

* any other auth plugin supported by MySQL.
## Attributes Reference

The following attributes are exported:

* `user` - The name of the user.
* `password` - The password of the user.
* `id` - The id of the user created, composed as "username@host".
* `host` - The host where the user was created.

## Attributes Reference

No further attributes are exported.

## Import

Users can be imported using user and host.

```
$ terraform import mysql_user.example user@host
```
