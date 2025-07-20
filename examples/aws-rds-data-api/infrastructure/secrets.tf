# Secrets Manager secret for database credentials

resource "aws_secretsmanager_secret" "db_credentials" {
  name = "${var.cluster_name}-db-credentials"

  tags = {
    Name        = "${var.cluster_name}-db-credentials"
    Environment = var.environment
  }
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = aws_secretsmanager_secret.db_credentials.id
  secret_string = jsonencode({
    username = var.master_username
    password = var.master_password
  })
}