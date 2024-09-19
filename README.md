zph/terraform-provider-mysql

# Purpose

zph fork of terraform-provider-mysql exists for the following goals:
1. To design and trial TiDB integrations or patches before committing upstream
2. To validate design changes for the project before offering upstream.

Changes here are intended to be upstreamed to petoju's fork to avoid ecosystem
fragmentation. We will update this readme if those design choices change.

## Release Naming

zph fork will use release naming in the following form:

v3.0.62005

{petoju version}{sequential int of additional applied patches}

This indicates that the base is v3.0.62 from petoju, with modifications from zph
repo that are 5 patch sets added.

It allows for keeping patches alive on this fork until they land upstream and are
released there.

## Security / Chain of Custody

We sign releases with a GPG key currently using goreleaser locally on the personal
equipment of @ZPH. As the maintainer of this fork, I, @ZPH, attest that the builds
represent the exact SHA of the version control with no alterations. The credentials
are stored in a credential manager with layers of safeguards and no other individuals
have access.

The near term goal is to setup github actions to provide this guarantee
so that even if I were a malicious actor or coerced,
I could not introduce opaque security issues into binary releases.

In the meantime, I certify by my professional reputation and career as:
https://www.linkedin.com/in/zph/ that appropriate safeguards are being taken.

## Original Readme
Below is from petoju/terraform-provider-mysql:

**This repository is an unofficial fork**

The fork is mostly based of the official (now archived) repo.
The provider has also some extra changes and solves almost all the reported
issues.

I incorporated changes by winebarrel/terraform-provider-mysql. Because I found
out about that repo only after some work, PR cannot be created easily now.
Feel free to create a PR with my code to their repo or the other way around.

[![Build Status](https://www.travis-ci.com/petoju/terraform-provider-mysql.svg?branch=master)](https://www.travis-ci.com/petoju/terraform-provider-mysql)

Terraform Provider
==================

Requirements
------------

-	[Terraform](https://www.terraform.io/downloads.html) 0.12.x
-	[Go](https://golang.org/doc/install) 1.17 (to build the provider plugin)

Usage
-----

For Terraform 0.12+ compatibility, the configuration should specify version 1.6 or higher:

```hcl
provider "mysql" {
  version = "~> 1.6"
}
```

Building The Provider
---------------------

If you want to reproduce a build (to verify my build confirms to sources),
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
   1. Then add it to your path OR `brew link mysql-client@8.0`

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
