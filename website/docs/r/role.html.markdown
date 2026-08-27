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
