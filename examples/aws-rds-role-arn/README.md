# AWS RDS IAM Authentication with Role ARN

This example demonstrates how to use the terraform-provider-mysql with AWS RDS IAM authentication using assume role functionality.

## Prerequisites

1. AWS RDS MySQL/MariaDB instance with IAM authentication enabled
2. IAM role with permissions to connect to the RDS instance
3. IAM user/role with permissions to assume the target role
4. Database user created with IAM authentication enabled

## Setup

### 1. Enable IAM Authentication on RDS Instance

Ensure your RDS instance has IAM authentication enabled:

```bash
aws rds modify-db-instance \
    --db-instance-identifier your-instance-id \
    --enable-iam-database-authentication \
    --apply-immediately
```

### 2. Create IAM Role

Create an IAM role that can connect to your RDS instance:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "rds-db:connect"
            ],
            "Resource": [
                "arn:aws:rds-db:us-east-1:123456789012:dbuser:your-db-resource-id/your-iam-database-user"
            ]
        }
    ]
}
```

### 3. Create Database User

Connect to your RDS instance and create a user for IAM authentication:

```sql
CREATE USER 'your-iam-database-user'@'%' IDENTIFIED WITH AWSAuthenticationPlugin as 'RDS';
GRANT ALL PRIVILEGES ON *.* TO 'your-iam-database-user'@'%';
FLUSH PRIVILEGES;
```

### 4. Configure Trust Relationship

Ensure your IAM role has a trust relationship that allows the entity running Terraform to assume it:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::123456789012:user/your-terraform-user"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

## Usage

1. Update the configuration in `main.tf` with your specific values:
   - Replace `your-rds-endpoint.amazonaws.com` with your RDS endpoint
   - Replace `your-iam-database-user` with your IAM database user
   - Replace `arn:aws:iam::123456789012:role/MyRDSRole` with your role ARN
   - Update the region as needed

2. Run Terraform:

```bash
terraform init
terraform plan
terraform apply
```

## Configuration Options

The provider supports several ways to configure AWS credentials with assume role:

### Option 1: Default credentials with assume role
```hcl
provider "mysql" {
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    aws_rds_iam_auth = true
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}
```

### Option 2: Static credentials with assume role
```hcl
provider "mysql" {
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
```

### Option 3: AWS profile with assume role
```hcl
provider "mysql" {
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    aws_rds_iam_auth = true
    profile          = "your-aws-profile"
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}
```

### Option 4: Legacy usage with aws:// prefix
```hcl
provider "mysql" {
  endpoint = "aws://your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-database-user"
  
  aws_config {
    region   = "us-east-1"
    role_arn = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}
```

## Benefits of Using Role ARN

- **Enhanced Security**: Use temporary credentials from assumed roles instead of long-term access keys
- **Cross-Account Access**: Assume roles in different AWS accounts
- **Principle of Least Privilege**: Grant minimal permissions to the base credentials and use assume role for specific operations
- **Compatibility**: Works similar to the PostgreSQL provider's `aws_rds_iam_provider_role_arn` parameter

## Troubleshooting

1. **InvalidClientTokenId**: Ensure your base credentials have permission to assume the target role
2. **Access Denied**: Check that the assumed role has the correct RDS connect permissions
3. **Database Connection Failed**: Verify the database user exists and has IAM authentication enabled
4. **Incorrect Role ARN**: Ensure the role ARN format is correct and the role exists