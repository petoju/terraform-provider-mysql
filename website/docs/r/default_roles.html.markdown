---
layout: "mysql"
page_title: "MySQL: mysql_default_roles"
sidebar_current: "docs-mysql-default-roles"
description: |-
  Creates and manages a user's default roles on a MySQL server.
---

# mysql\_default_roles

The ``mysql_default_roles`` resource creates and manages a user's default roles on a MySQL server.

~> **Note:** This resource is available on MySQL version 8.0.0 and later.

## Example Usage

```hcl
resource "mysql_role" "readonly" {
  name = "readonly"
}

resource "mysql_user" "jdoe" {
  user = "jdoe"
  host = "%"
}

resource "mysql_grant" "jdoe" {
  user     = mysql_user.jdoe.user
  host     = mysql_user.jdoe.host
  database = ""
  roles    = [mysql_role.readonly.name]
}

resource "mysql_default_roles" "jdoe" {
  user  = mysql_user.jdoe.user
  host  = mysql_user.jdoe.host
  roles = mysql_grant.jdoe.roles
}
```

## Argument Reference

The following arguments are supported:

* `user` - (Required) The name of the user.
* `host` - (Optional) The source host of the user. Defaults to "localhost".
* `roles` - (Optional) A list of default roles to assign to the user. By default no roles are assigned.

~> **Note:** Creating a new default roles resource on an existing user will **overwrite** the user's existing default roles. Likewise, destryoing a default roles resource will **remove** the user's default roles, equivalent to running `ALTER USER ... DEFAULT ROLE NONE`.

## Attributes Reference

The following attributes are exported:

* `id` - The id of the user default roles created, composed as "username@host".
* `user` - The name of the user.
* `host` - The host where the user was created.
* `roles` - The default roles assigned to the user.

## Import

User default roles can be imported using user and host.

```shell
terraform import mysql_default_roles.example user@host
```
