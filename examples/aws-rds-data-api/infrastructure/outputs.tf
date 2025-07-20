output "cluster_arn" {
  value       = aws_rds_cluster.serverless.arn
  description = "ARN of the Aurora Serverless cluster"
}

output "cluster_endpoint" {
  value       = aws_rds_cluster.serverless.endpoint
  description = "Cluster endpoint (not used with Data API)"
}

output "secret_arn" {
  value       = aws_secretsmanager_secret.db_credentials.arn
  description = "ARN of the Secrets Manager secret"
}

output "region" {
  value       = var.region
  description = "AWS region where resources were created"
}