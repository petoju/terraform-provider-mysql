---
layout: "mysql"
page_title: "MySQL: mysql_grant"
sidebar_current: "docs-mysql-resource-grant"
description: |-
  Creates and manages privileges given to a user on a MySQL server
---

# mysql\_grant

The ``mysql_grant`` resource creates and manages privileges given to
a user on a MySQL server.

## Granting Privileges to a User

```hcl
resource "mysql_user" "jdoe" {
  user               = "jdoe"
  host               = "example.com"
  plaintext_password = "password"
}

resource "mysql_grant" "jdoe" {
  user       = mysql_user.jdoe.user
  host       = mysql_user.jdoe.host
  database   = "app"
  privileges = ["SELECT", "UPDATE"]
}
```

## Granting Privileges to a Role

```hcl
resource "mysql_role" "developer" {
  name = "developer"
}

resource "mysql_grant" "developer" {
  role       = mysql_role.developer.name
  database   = "app"
  privileges = ["SELECT", "UPDATE"]
}
```

## Adding a Role to a User

```hcl
resource "mysql_user" "jdoe" {
  user               = "jdoe"
  host               = "example.com"
  plaintext_password = "password"
}

resource "mysql_role" "developer" {
  name = "developer"
}

resource "mysql_grant" "developer" {
  user     = mysql_user.jdoe.user
  host     = mysql_user.jdoe.host
  database = "app"                   #Optional
  roles    = [mysql_role.developer.name]
}
```

## Argument Reference

~> **Note:** MySQL removed the `REQUIRE` option from `GRANT` in version 8. `tls_option` is ignored in MySQL 8 and above.

~> **Note:** Attributes `role` and `roles` are only supported in MySQL 8 and above.

The following arguments are supported:

* `user` - (Optional) The name of the user. Conflicts with `role`.
* `host` - (Optional) The source host of the user. Defaults to "localhost". Conflicts with `role`.
* `role` - (Optional) The role to grant `privileges` to. Conflicts with `user` and `host`.
* `database` - (Required) The database to grant privileges on.
* `table` - (Optional) Which table to grant `privileges` on. Defaults to `*`, which is all tables.
* `privileges` - (Optional) A list of privileges to grant to the user. Refer to a list of privileges (such as [here](https://dev.mysql.com/doc/refman/5.5/en/grant.html)) for applicable privileges. Conflicts with `roles`.
* `roles` - (Optional) A list of roles to grant to the user. Conflicts with `privileges`.
* `tls_option` - (Optional) An TLS-Option for the `GRANT` statement. The value is suffixed to `REQUIRE`. A value of 'SSL' will generate a `GRANT ... REQUIRE SSL` statement. See the [MYSQL `GRANT` documentation](https://dev.mysql.com/doc/refman/5.7/en/grant.html) for more. Ignored if MySQL version is under 5.7.0.
* `grant` - (Optional) Whether to also give the user privileges to grant the same privileges to other users.

## Attributes Reference

No further attributes are exported.

## Import

Grants can be imported using user, host, database and table.
For grants without explicit database or tables, leave these fields empty.

You can also add an extra at sign `@` to the import definition to specify
the grant contains WITH GRANT OPTION.

```
$ terraform import mysql_grant.example user@host@database@table
$ terraform import mysql_grant.without_db user@host@@

# Import the first example with grant option
$ terraform import mysql_grant.example user@host@database@table@
```
