---
layout: "mysql"
page_title: "MySQL: mysql_cloud_sql_audit_rule"
sidebar_current: "docs-mysql-cloud-sql-audit-rule"
description: |-
  Creates and manages audit rule on a Google Cloud SQL for MySQL Server
---

# mysql\_cloud_sql_audit_rule

The ``mysql_cloud_sql_audit_rule`` resource creates and manages audit rules on a Google Cloud SQL for MySQL server.

See also: https://docs.cloud.google.com/sql/docs/mysql/use-db-audit

## Example Usage

```hcl

resource "mysql_user" "jdoe" {
  user = "jdoe"
  host = "%"
}

resource "mysql_cloud_sql_audit_rule" "jdoe" {
  username  = [mysql_user.jdoe.user]
  database  = ["db1", "db2"]
  object    = ["table1", "table2"]
  operation = ["select", "update"]
  op_result = "B"
}
```

## Argument Reference

The following arguments are supported:

* `username` - (Required) A list of users, can we in the form of `user@host` or `username`, when host is omited, `*` is presumed to be the host.
* `database` - (Required) A list of databases to be audited, set to `['*']` to audit all databases.
* `object` - (Request) A list of objects (tables) to be audited, set to `['*']` to audit all objects.
* `operation` - (Required) A list of operations to be audited, set to `['*']` to audit all operations. See [google's documentation](https://docs.cloud.google.com/sql/docs/mysql/db-audit-operations) for a list of supported operations.
* `op_result` - (Required) The operation result for which to enable this audit rule: success (S), unsuccessful (U) or both (B) successful and unsuccessful operations.

~> **Note:** Creating a new default roles resource on an existing user will **overwrite** the user's existing default roles. Likewise, destryoing a default roles resource will **remove** the user's default roles, equivalent to running `ALTER USER ... DEFAULT ROLE NONE`.

## Attributes Reference

The following attributes are exported:

* `username` - A list of users for whom the audit rule is active.
* `database` - A list of databases for which the audit is active.
* `object` - A list of objects for which the audit rule is active
* `operation` - A list of operations for which the audit rule is active.
* `op_result` - The operation result for which this audit rule is active.

## Import

Audit rules can be imported by their rule ID, these rule IDs can be found by calling `mysql.cloudsql_list_audit_rule` stored procedure.

```sql
CALL mysql.cloudsql_list_audit_rule('*',@outval,@outmsg);
```

After recovering the correct rule ID, the resource can be imported as follows:

```shell
terraform import mysql_cloud_sql_audit_rule.example 1
```
