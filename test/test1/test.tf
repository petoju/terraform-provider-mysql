terraform {
  required_providers {
    mysql = {
      source  = "petoju/mysql"
      version = ">= 3.0.37"
    }
  }
  required_version = ">= 1.11.4"
}

variable "MYSQL_ROOT_USER" {
  sensitive = true
  type      = string
}

variable "MYSQL_ROOT_PASSWORD" {
  sensitive = true
  type      = string
}

# First provider

provider "mysql" {
  alias    = "local1"
  endpoint = "localhost:3307"
  username = var.MYSQL_ROOT_USER
  password = var.MYSQL_ROOT_PASSWORD
}

resource "mysql_database" "test1" {
  provider = mysql.local1
  name     = "test-db"
}


resource "mysql_role" "analyst1" {
  provider = mysql.local1
  name     = "analyst"
}

resource "mysql_grant" "grant_to_role_select1" {
  provider   = mysql.local1
  role       = mysql_role.analyst1.name
  database   = mysql_database.test1.name
  table      = "*"
  privileges = ["SELECT"]
}

resource "mysql_grant" "grant_to_role_showroutine1" {
  provider   = mysql.local1
  role       = mysql_role.analyst1.name
  database   = "*"
  table      = "*"
  privileges = ["SHOW_ROUTINE"]
}

resource "mysql_grant" "grant_to_user1" {
  provider   = mysql.local1
  user       = mysql_user.user1.user
  host       = "%"
  database   = mysql_database.test1.name
  roles      = [mysql_role.analyst1.name]
}


resource "mysql_user" "user1" {
  provider   = mysql.local1
  user       = "test-user"
  host	     = "%"
}

# Second provider

provider "mysql" {
  alias    = "local2"
  endpoint = "localhost:3308"
  username = var.MYSQL_ROOT_USER
  password = var.MYSQL_ROOT_PASSWORD
}

resource "mysql_database" "test2" {
  provider = mysql.local2
  name     = "test-db"
}


resource "mysql_role" "analyst2" {
  provider = mysql.local2
  name     = "analyst"
}

resource "mysql_grant" "grant_to_role_select2" {
  provider   = mysql.local2
  role       = mysql_role.analyst2.name
  database   = mysql_database.test2.name
  table      = "*"
  privileges = ["SELECT"]
}

resource "mysql_grant" "grant_to_role_showroutine2" {
  provider   = mysql.local2
  role       = mysql_role.analyst2.name
  database   = "*"
  table      = "*"
  privileges = ["SHOW_ROUTINE"]
}

resource "mysql_grant" "grant_to_user2" {
  provider   = mysql.local2
  user       = mysql_user.user2.user
  host       = "%"
  database   = mysql_database.test2.name
  roles      = [mysql_role.analyst2.name]
}


resource "mysql_user" "user2" {
  provider   = mysql.local2
  user       = "test-user"
  host	     = "%"
}
