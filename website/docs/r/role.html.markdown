---
layout: "mysql"
page_title: "MySQL: mysql_role"
sidebar_current: "docs-mysql-resource-role"
description: |-
  Creates and manages a role  on a MySQL server.
---

# mysql\_role

The ``mysql_role`` resource creates and manages a user on a MySQL
server.

~> **Note:** MySQL introduced roles in version 8. They do not work on MySQL 5 and lower.

## Example Usage

```hcl
resource "mysql_role" "developer" {
  name = "developer"
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the role.

## Attributes Reference

No further attributes are exported.

## Import

Roles can be imported using their name, e.g.

```
$ terraform import mysql_role.example my-role
```

Import refuses names that belong to a login user rather than a role, so importing the wrong
name fails with a clear error instead of quietly bringing a user under `mysql_role`
management.

On MariaDB this is exact, using `mysql.user.is_role`. On MySQL, Percona and TiDB roles and
users are the same kind of object, so two signals are combined: the state `CREATE ROLE`
leaves behind (a locked account with no password), and whether the account appears in
`mysql.role_edges`, which only happens once it has been granted to somebody. The second
signal matters for a role that was later given a password or unlocked, which the first would
otherwise mistake for a user.

The check is best-effort: it reads `mysql.user`, and on MySQL-compatible servers also
`mysql.role_edges`. Where those are not readable the import proceeds with a warning in the
logs rather than failing, so verify the name is a role yourself in that case.
