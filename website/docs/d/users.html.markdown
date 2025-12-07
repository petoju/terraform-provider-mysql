---
layout: "mysql"
page_title: "MYSQL: mysql_users"
sidebar_current: "docs-mysql-datasource-users"
description: |-
  Gets users on a MySQL server.
---

# Data Source: mysql_users

The `mysql_users` gets users on a MySQL server.

## Example Usage

```hcl
data "mysql_users" "example" {
  user_pattern  = "app_user_%"
  host_pattern  = "localhost"
  exclude_users = ["app_user_admin@localhost"]
}
```

## Argument Reference

The following arguments are supported:

- `user_pattern` - (Optional) The filter applied to the user name using a SQL LIKE pattern.
- `host_pattern` - (Optional) The filter applied to the host using a SQL LIKE pattern.
- `exclude_users` - (Optional) The list of users to exclude from the results. Each value should be in the format `user@host`.

## Attributes Reference

The following attributes are exported:

- `users` - A set of objects, each representing a MySQL user. Each object contains the following keys:
  - `user`
  - `host`
