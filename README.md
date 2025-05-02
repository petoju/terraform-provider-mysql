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
