terraform {
  required_version = ">= 1.5.7"

  required_providers {
    mongodb = {
      source = "registry.terraform.io/zph/mysql"
      version = "9.9.9"
    }
  }
}

provider "mysql" {
  endpoint = "localhost:4000"
  username = "root"
  #alias = "tidb"
  #password = "admin"
}
