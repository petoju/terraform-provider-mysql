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

On MariaDB, importing a name that belongs to a login user rather than a role fails with a
clear error, using `mysql.user.is_role`.

MySQL, Percona and TiDB do not distinguish roles from users at engine level - both are rows
in `mysql.user`, and a role can be given a password and unlocked like any other account.
There the provider can only confirm a role positively: by the state `CREATE ROLE` leaves
behind (a locked account with no password), or by the account appearing in
`mysql.role_edges`, which happens once it has been granted to somebody.

~> **Note:** A name that cannot be confirmed either way is imported anyway, with a warning
logged, rather than being refused - so on those servers importing a name that is really a
user will succeed. Make sure the name you pass is a role.
