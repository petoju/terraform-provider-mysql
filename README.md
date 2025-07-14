**This repository is an unofficial fork**

The fork is mostly based on the official (now archived) repo.
The provider also includes some extra changes and resolves almost all the
reported issues.

I incorporated changes from [winebarrel/terraform-provider-mysql](https://github.com/winebarrel/terraform-provider-mysql),
another fork from the official repo.

[![Build Status](https://www.travis-ci.com/petoju/terraform-provider-mysql.svg?branch=master)](https://www.travis-ci.com/petoju/terraform-provider-mysql)

Terraform Provider
==================

Requirements
------------

-	[Terraform](https://www.terraform.io/downloads.html) 0.12.x
-	[Go](https://golang.org/doc/install) 1.17 (to build the provider plugin)

Usage
-----

Just include the provider, example:

```hcl
terraform {
  required_providers {
    mysql = {
      source = "petoju/mysql"
      version = "~> 3.0.72"
    }
  }
}
```

Building The Provider
---------------------

If you want to reproduce a build (to verify that my build conforms to the sources),
download the provider of any version first and find the correct go version:
```
egrep -a -o 'go1[0-9\.]+' path_to_the_provider_binary
```

Clone the repository anywhere. Use `goreleaser` to build the packages for all architectures:
```
goreleaser build --clean
```

Files in dist should match whatever is provided. If they don't, consider reading
https://words.filippo.io/reproducing-go-binaries-byte-by-byte/ or open an issue here.

There is also experimental way to build everything in docker. I will try to use it every time,
but I may skip it if it doesn't work. That should roughly match how I build the provider locally.

Using the provider
----------------------

### AWS RDS IAM Authentication

The provider supports AWS RDS IAM authentication using the `aws_rds_iam_auth` parameter. You can configure AWS credentials and assume role settings using the `aws_config` block.

#### Prerequisites:

Before using AWS RDS IAM authentication, ensure:

1. **RDS Instance**: IAM authentication is enabled on your RDS instance
2. **Database User**: Create user with IAM plugin: `CREATE USER 'username' IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';`
3. **IAM Permissions**: Your AWS credentials have `rds-db:connect` permission
4. **Network**: Security groups allow connection from your Terraform execution environment

#### Basic usage with AWS RDS IAM authentication:

```hcl
provider "mysql" {
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-user"
  
  aws_config {
    aws_rds_iam_auth = true
    region           = "us-east-1"
  }
}
```

#### Using assume role for AWS RDS IAM authentication:

```hcl
provider "mysql" {
  endpoint = "your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-user"
  
  aws_config {
    aws_rds_iam_auth = true
    region           = "us-east-1"
    role_arn         = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}
```

#### Legacy usage (backward compatibility):

For backward compatibility, the `aws://` endpoint prefix is still supported:

```hcl
provider "mysql" {
  endpoint = "aws://your-rds-endpoint.amazonaws.com:3306"
  username = "your-iam-user"
  
  aws_config {
    region   = "us-east-1"
    role_arn = "arn:aws:iam::123456789012:role/MyRDSRole"
  }
}
```

#### Available aws_config parameters:

- `region` - AWS region where the RDS instance is located
- `profile` - AWS profile to use from credentials file
- `access_key` - AWS access key (must be used with secret_key)
- `secret_key` - AWS secret key (must be used with access_key)  
- `role_arn` - ARN of the IAM role to assume for RDS authentication

#### Important notes:

- When `aws_rds_iam_auth = true` is set in the `aws_config` block, the `password` parameter is ignored and auth token is generated automatically
- The `role_arn` parameter allows you to assume a specific IAM role for RDS authentication, similar to the PostgreSQL provider functionality
- The database user must be created with IAM authentication enabled: `CREATE USER 'username' IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';`
- IAM database authentication must be enabled on your RDS instance
- Your AWS credentials must have `rds-db:connect` permission for the specific database user and instance
- TLS connection is required for AWS RDS IAM authentication (ensure your `tls` parameter is properly configured)

## Fill in for each provider

Developing the Provider
---------------------------

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (version 1.17+ is *required*). You'll also need to correctly setup a [GOPATH](http://golang.org/doc/code.html#GOPATH), as well as adding `$GOPATH/bin` to your `$PATH`.

To compile the provider, run `make build`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

```sh
$ make bin
...
$ $GOPATH/bin/terraform-provider-mysql
...
```
### Ensure local requirements are present:

1. Docker environment
2. mysql-client binary which can be installed on Mac with `brew install mysql-client@8.0`
   1. Then add it to your path OR run `brew link mysql-client@8.0`

### Running tests

In order to test the provider, you can simply run `make test`.

```sh
$ make test
```

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```sh
$ make testacc
```

If you want to run the Acceptance tests on your own machine with a MySQL in Docker:

```bash
make acceptance
# or to test only one mysql version:
make testversion8.0
```
