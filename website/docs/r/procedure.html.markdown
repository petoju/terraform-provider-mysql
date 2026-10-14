---
layout: "mysql"
page_title: "MySQL: mysql_procedure"
sidebar_current: "docs-mysql-resource-procedure"
description: |-
  Creates and manages a stored procedure on a MySQL server.
---

# mysql\_procedure

The ``mysql_procedure`` resource creates and manages a stored procedure on a
MySQL server.

~> **Note:** TiDB does not implement stored procedures, so this resource cannot
be used against it.

## Example Usage

```hcl
resource "mysql_database" "app" {
  name = "my_awesome_app"
}

resource "mysql_procedure" "greet" {
  database = mysql_database.app.name
  name     = "greet"

  parameter {
    name = "person"
    type = "VARCHAR(50)"
  }

  parameter {
    mode = "OUT"
    name = "greeting"
    type = "VARCHAR(100)"
  }

  comment         = "Greets a person"
  deterministic   = true
  sql_data_access = "NO SQL"

  body = <<-SQL
    BEGIN
      SET greeting = CONCAT('Hello, ', person);
    END
  SQL
}
```

## Argument Reference

The following arguments are supported:

* `database` - (Required) The database to create the procedure in. Changing
  this forces the procedure to be recreated.

* `name` - (Required) The name of the procedure. Changing this forces the
  procedure to be recreated.

* `body` - (Required) The routine body, most commonly a ``BEGIN ... END``
  block. Leading and trailing whitespace and a trailing `;` are stripped,
  matching what the server stores. The server does not store a comment after
  the final `;` either, so such a comment causes a permanent diff. Changing
  this recreates the procedure in place, because MySQL cannot alter the body
  of an existing routine. See the note on recreation below.

* `parameter` - (Optional) A parameter of the procedure, in declaration order.
  May be given more than once. Changing the parameters recreates the
  procedure in place. The block supports:
  * `name` - (Required) The parameter name.
  * `type` - (Required) The parameter type, written exactly as it would be in
    the ``CREATE PROCEDURE`` statement, for example ``INT``,
    ``DECIMAL(10,2)`` or ``ENUM('a','b')``.
  * `mode` - (Optional) One of ``IN``, ``OUT`` or ``INOUT``. Defaults to
    ``IN``.

* `definer` - (Optional) The account the procedure is attributed to, in
  ``user@host`` form. Defaults to the account Terraform connects with.
  Specifying another account requires the ``SET_USER_ID`` privilege (``SUPER``
  before MySQL 8.0.16), which is not granted on managed offerings such as RDS.
  Changing this recreates the procedure in place.

* `comment` - (Optional) A comment describing the procedure. Defaults to an
  empty string.

* `deterministic` - (Optional) Whether the procedure always produces the same
  result for the same input. Defaults to ``false``. Changing this recreates
  the procedure in place, because ``ALTER PROCEDURE`` cannot change it.

* `sql_data_access` - (Optional) What kind of SQL the procedure contains, one
  of ``CONTAINS SQL``, ``NO SQL``, ``READS SQL DATA`` or
  ``MODIFIES SQL DATA``. Defaults to ``CONTAINS SQL``. This is advisory only,
  the server does not enforce it.

* `security_type` - (Optional) Whether the procedure executes with the
  privileges of its ``DEFINER`` or of the ``INVOKER`` calling it. Defaults to
  ``DEFINER``.

~> **Note:** `body` and the parameter `type` are interpolated into the
``CREATE PROCEDURE`` statement as written, so they must come from a trusted
source.

~> **Note:** A change to `body`, `parameter`, `definer` or `deterministic`
drops the procedure and creates it again in the same apply. With the default
``automatic_sp_privileges = ON``, ``DROP PROCEDURE`` revokes the privileges of
every account on the procedure. The provider therefore reads them from
``mysql.procs_priv`` before the ``DROP`` and grants them again after the
``CREATE``, so `mysql_grant` resources on the procedure keep working. This
needs ``SELECT`` on ``mysql.procs_priv``; without it, the update stops before
the ``DROP``. If the ``CREATE`` fails, the provider restores the previous
definition and its privileges. If the restore fails too, for example because
the account cannot set the configured `definer`, the procedure stays dropped
and the error says so. Calls between the ``DROP`` and the ``CREATE`` fail. If
`definer` is not configured, the ``CREATE`` has no ``DEFINER`` clause, so the
account Terraform connects with becomes the definer, as on the first create.
On MariaDB, the provider uses ``CREATE OR REPLACE PROCEDURE``, which keeps the
privileges and the old procedure if it fails.

## Attributes Reference

The following attributes are exported:

* `id` - The database and the name of the procedure, separated by a dot, e.g.
  ``my_awesome_app.greet``.
* `definer` - The account the procedure is attributed to.

## Import

Procedures can be imported using the database and the procedure name separated
by a dot, e.g.

```
$ terraform import mysql_procedure.greet my_awesome_app.greet
```

Importing a procedure that was not created by Terraform reads the parameter
list and the body back exactly as they were declared on the server, so the imported
configuration may need reformatting to match your style.

~> **Note:** The dot separating the two parts is the first one in the ID, so
procedures living in a database whose name contains a dot cannot be imported.
