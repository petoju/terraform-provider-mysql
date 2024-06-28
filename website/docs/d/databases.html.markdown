---
layout: "mysql"
page_title: "MySQL: mysql_databases"
sidebar_current: "docs-mysql-datasource-databases"
description: |-
  Gets databases on a MySQL server.
---

# Data Source: mysql\_databases

The ``mysql_databases`` gets databases on a MySQL
server.

## Example Usage

```hcl
data "mysql_databases" "app" {
  pattern = "test_%"
}
```

## Argument Reference

The following arguments are supported:

* `pattern` - (Optional) Patterns for searching databases.

## Attributes Reference

The following attributes are exported:

* `databases` - The list of the database names.
