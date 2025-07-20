variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "cluster_name" {
  description = "Name for the Aurora Serverless cluster"
  type        = string
  default     = "mysql-data-api-test"
}

variable "database_name" {
  description = "Name of the initial database"
  type        = string
  default     = "testdb"
}

variable "master_username" {
  description = "Master username for the database"
  type        = string
  default     = "admin"
}

variable "master_password" {
  description = "Master password for the database"
  type        = string
  sensitive   = true
  default     = "hunter2password"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "test"
}
