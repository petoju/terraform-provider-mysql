resource "mysql_database" "app_db" {
  provider = mysql.rds_data_api
  name     = "app_database"
}

resource "mysql_database" "analytics_db" {
  provider              = mysql.rds_data_api
  name                  = "analytics_database"
  default_character_set = "utf8mb4"
  default_collation     = "utf8mb4_unicode_ci"
}

resource "mysql_user" "app_user" {
  provider = mysql.rds_data_api
  user     = "app_user"
  host     = "%"

  plaintext_password = "AppUser123!"

  depends_on = [mysql_database.app_db]
}

resource "mysql_user" "readonly_user" {
  provider = mysql.rds_data_api
  user     = "readonly_user"
  host     = "%"

  plaintext_password = "ReadOnly123!"

  depends_on = [mysql_database.app_db]
}

# Example of mysql_user with AWS IAM authentication
resource "mysql_user" "iam_user" {
  provider    = mysql.rds_data_api
  user        = "iam_db_user"
  host        = "%"
  auth_plugin = "AWSAuthenticationPlugin"

  depends_on = [mysql_database.app_db]
}

# Grant privileges to IAM authenticated user
resource "mysql_grant" "iam_user_grant" {
  provider = mysql.rds_data_api
  user     = mysql_user.iam_user.user
  host     = mysql_user.iam_user.host
  database = mysql_database.app_db.name

  privileges = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "mysql_grant" "app_user_grant" {
  provider = mysql.rds_data_api
  user     = mysql_user.app_user.user
  host     = mysql_user.app_user.host
  database = mysql_database.app_db.name

  privileges = ["SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "INDEX", "ALTER"]
}

resource "mysql_grant" "readonly_grant" {
  provider = mysql.rds_data_api
  user     = mysql_user.readonly_user.user
  host     = mysql_user.readonly_user.host
  database = mysql_database.app_db.name

  privileges = ["SELECT"]
}

resource "mysql_role" "developer_role" {
  provider = mysql.rds_data_api
  name     = "developer"

}

resource "mysql_role" "analyst_role" {
  provider = mysql.rds_data_api
  name     = "analyst"

}

# Grant privileges to roles
resource "mysql_grant" "developer_role_grant" {
  provider = mysql.rds_data_api
  role     = mysql_role.developer_role.name
  database = mysql_database.app_db.name

  privileges = ["ALL PRIVILEGES"]
}

resource "mysql_grant" "analyst_role_grant" {
  provider = mysql.rds_data_api
  role     = mysql_role.analyst_role.name
  database = mysql_database.analytics_db.name

  privileges = ["SELECT", "SHOW VIEW"]
}

# Grant the developer role to app_user
resource "mysql_grant" "app_user_developer_grant" {
  provider = mysql.rds_data_api
  user     = mysql_user.app_user.user
  host     = mysql_user.app_user.host
  database = "*"
  roles    = [mysql_role.developer_role.name]
}

# Example of mysql_default_roles resource
# Note: Roles must be granted to users before they can be set as default
resource "mysql_default_roles" "app_user_roles" {
  provider = mysql.rds_data_api
  user     = mysql_user.app_user.user
  host     = mysql_user.app_user.host

  roles = [
    mysql_role.developer_role.name
  ]

  depends_on = [mysql_grant.app_user_developer_grant]
}

# Example of mysql_rds_config resource for RDS-specific settings
resource "mysql_rds_config" "binlog_retention" {
  provider               = mysql.rds_data_api
  binlog_retention_hours = 24
}

# Create the rotating_user first
resource "mysql_user" "rotating_user" {
  provider           = mysql.rds_data_api
  user               = "rotating_user"
  host               = "%"
  plaintext_password = "InitialPassword123!"
}

# Example of mysql_user_password resource for managing password separately
# Note: This resource expects the user to already exist
resource "mysql_user_password" "rotating_user" {
  provider = mysql.rds_data_api
  user     = "rotating_user"
  host     = "%"

  plaintext_password = "NewPassword456!"

  depends_on = [mysql_user.rotating_user]
}

# Output connection information
output "mysql_resources_created" {
  value = {
    databases = [
      mysql_database.app_db.name,
      mysql_database.analytics_db.name
    ]
    users = [
      mysql_user.app_user.user,
      mysql_user.readonly_user.user,
      mysql_user.rotating_user.user,
      mysql_user.iam_user.user
    ]
    roles = [
      mysql_role.developer_role.name,
      mysql_role.analyst_role.name
    ]
  }

  depends_on = [
    mysql_database.app_db,
    mysql_database.analytics_db,
    mysql_user.app_user,
    mysql_user.readonly_user,
    mysql_user.iam_user,
    mysql_role.developer_role,
    mysql_role.analyst_role
  ]
}
