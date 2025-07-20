# Aurora Serverless v2 cluster with Data API enabled

resource "aws_rds_cluster" "serverless" {
  cluster_identifier      = var.cluster_name
  engine                  = "aurora-mysql"
  engine_version          = "8.0.mysql_aurora.3.09.0"
  database_name           = var.database_name
  master_username         = var.master_username
  master_password         = var.master_password
  backup_retention_period = 1
  skip_final_snapshot     = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.aurora.id]

  # Enable Data API
  enable_http_endpoint = true

  serverlessv2_scaling_configuration {
    min_capacity = 0.5
    max_capacity = 1.0
  }

  tags = {
    Name        = var.cluster_name
    Environment = var.environment
  }
}

# Aurora Serverless v2 requires at least one instance
resource "aws_rds_cluster_instance" "serverless" {
  identifier         = "${var.cluster_name}-instance-1"
  cluster_identifier = aws_rds_cluster.serverless.id
  instance_class     = "db.serverless"
  engine             = aws_rds_cluster.serverless.engine
  engine_version     = aws_rds_cluster.serverless.engine_version

  tags = {
    Name        = "${var.cluster_name}-instance-1"
    Environment = var.environment
  }
}
