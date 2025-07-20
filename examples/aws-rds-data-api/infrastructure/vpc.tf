# VPC and networking resources for Aurora Serverless

data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "${var.cluster_name}-vpc"
    Environment = var.environment
  }
}

resource "aws_subnet" "private" {
  count             = 3
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 1}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name        = "${var.cluster_name}-private-${count.index + 1}"
    Environment = var.environment
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.cluster_name}-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name        = "${var.cluster_name}-subnet-group"
    Environment = var.environment
  }
}

resource "aws_security_group" "aurora" {
  name        = "${var.cluster_name}-aurora-sg"
  description = "Security group for Aurora Serverless cluster"
  vpc_id      = aws_vpc.main.id

  # Aurora Serverless doesn't need inbound rules for Data API access
  # The Data API is accessed over HTTPS, not the MySQL port

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name        = "${var.cluster_name}-aurora-sg"
    Environment = var.environment
  }
}