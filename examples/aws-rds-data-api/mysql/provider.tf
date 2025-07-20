terraform {
  required_providers {
    mysql = {
      source  = "petoju/mysql"
      version = "~> 3.0"
    }
  }
}

# MySQL Provider configuration
# This provider uses the RDS Data API to connect to Aurora Serverless
provider "mysql" {
  alias = "rds_data_api"

  aws_config {
    use_rds_data_api = true
    region           = data.terraform_remote_state.infrastructure.outputs.region
    cluster_arn      = data.terraform_remote_state.infrastructure.outputs.cluster_arn
    secret_arn       = data.terraform_remote_state.infrastructure.outputs.secret_arn
  }
}
