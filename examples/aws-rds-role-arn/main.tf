terraform {
  required_providers {
    mysql = {
      source  = "petoju/mysql"
      version = "~> 3.0"
    }
  }
}

# Example: Using assume role for AWS RDS IAM authentication
provider "mysql" {
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    aws_rds_iam_auth = true
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}

# Example: Using assume role with existing credentials
provider "mysql" {
  alias    = "with_base_credentials"
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    access_key       = "your-access-key"
    aws_rds_iam_auth = true
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
    secret_key       = "your-secret-key"
  }
}

# Example: Using assume role with AWS profile
provider "mysql" {
  alias    = "with_profile"
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    aws_rds_iam_auth = true
    profile          = "your-aws-profile"
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}

# Example: Legacy usage with aws:// prefix (backward compatibility)
provider "mysql" {
  alias    = "legacy"
  endpoint = "aws://your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    region   = "us-east-1"
    role_arn = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}

# Create a database using the assume role provider
resource "mysql_database" "example" {
  name = "example_database"
}

# Create a user with the assume role provider
resource "mysql_user" "example" {
  user     = "example_user"
  host     = "%"
  password = "example_password"
}

# Grant privileges to the user
resource "mysql_grant" "example" {
  user       = mysql_user.example.user
  host       = mysql_user.example.host
  database   = mysql_database.example.name
  privileges = ["ALL"]
} 