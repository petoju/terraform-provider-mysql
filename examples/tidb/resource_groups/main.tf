terraform {
  required_version = ">= 1.5.7"

  required_providers {
    mysql = {
      source = "registry.terraform.io/zph/mysql"
      version = "9.9.9"
    }
  }
}

provider "mysql" {
  endpoint = "127.0.0.1:4000"

  username = "root"
}

resource "mysql_ti_resource_group" "rg1" {
  name = "rg1"
  resource_units = 2000
}

resource "mysql_ti_resource_group" "rg2" {
  name = "rg2"
  resource_units = 2000
  burstable = false
}

resource "mysql_ti_resource_group_user_assignment" "rg1_user1" {
  user = "user1"
  resource_group = mysql_ti_resource_group.rg1.name
}

resource "mysql_ti_resource_group_user_assignment" "rg111_rg" {
  user = "rg111"
  resource_group = mysql_ti_resource_group.rg1.name
}
