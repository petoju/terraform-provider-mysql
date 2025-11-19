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

## Release Process

The release process is automated through the `make release` command, which handles tag management, version file updates, GitHub releases, and pushing changes.

### Prerequisites

Before running the release process, ensure you have the following set up:

1. **GPG Key for Signing**: A GPG key must be configured for signing release artifacts.
   - Set the `GPG_FINGERPRINT` environment variable to your GPG key fingerprint:
     ```bash
     export GPG_FINGERPRINT="your-gpg-key-fingerprint"
     ```
   - To find your GPG key fingerprint:
     ```bash
     gpg --list-secret-keys --keyid-format LONG
     ```
   - The fingerprint is the long string after the key type (e.g., `RSA 4096/ABC123DEF456...`)

2. **GitHub Authentication**: You need authentication configured for pushing to GitHub.
   - Either configure SSH keys for git push, or
   - Set up a GitHub Personal Access Token with appropriate permissions
   - goreleaser will use `GITHUB_TOKEN` environment variable if set, otherwise it will use git credentials

3. **Required Tools**:
   - `goreleaser` - Install from https://goreleaser.com/install/
   - `git` - For version control operations
   - `make` - For running the release command

### Release Workflow

The `make release` command performs the following steps:

1. **Version Check**: Reads the current version from the `VERSION` file and checks if a tag for that version already exists.

2. **Tag Management**: 
   - If the tag already exists, it automatically suggests the next version by incrementing the build number (e.g., `v3.0.62006` → `v3.0.62007`)
   - You can accept the suggestion or enter a custom tag
   - If the tag doesn't exist, it uses the version from the `VERSION` file

3. **Version File Update**: If a new version was determined, the `VERSION` file is automatically updated with the new version number.

4. **Confirmation Prompts**: The process includes interactive confirmations at key steps:
   - Confirmation to create and tag the release
   - Confirmation to deploy as a GitHub release
   - Confirmation to push tags and commits to GitHub

5. **Tag Creation**: Creates an annotated git tag with the version number.

6. **Version File Commit**: If the `VERSION` file was modified, it commits the change with message "Update VERSION to {version}".

7. **GitHub Release**: Uses `goreleaser` to:
   - Build binaries for all supported platforms (Linux, macOS, Windows, FreeBSD)
   - Create archives (zip files) for each platform/architecture combination
   - Generate SHA256 checksums
   - Sign the checksum file with your GPG key
   - Create a draft GitHub release (you can publish it manually after review)

8. **Push to GitHub**: Pushes the tag and commits to the remote repository.

### Usage

To create a release, simply run:

```bash
make release
```

The process will guide you through each step with clear prompts. You can cancel at any point if needed.

### Example Release Session

```bash
$ make release
Checking if tag v3.0.62006 already exists...
Tag v3.0.62006 already exists!
Current version from VERSION file: 3.0.62006
Most recent tag: v3.0.62006 (version: 3.0.62006)

Suggested next tag: v3.0.62007
Enter next tag (or press Enter to use v3.0.62007): 

Updating VERSION file from 3.0.62006 to 3.0.62007...
VERSION file updated.

=========================================
Release Summary:
  Tag: v3.0.62007
  Version: 3.0.62007
=========================================

Do you want to create tag v3.0.62007? (yes/no): yes

Creating tag v3.0.62007...
VERSION file change committed.
Tag v3.0.62007 created successfully.

Do you want to deploy this as a GitHub release? (yes/no): yes

Running goreleaser to create GitHub release...
[... goreleaser output ...]

Do you want to push tags and commits to GitHub? (yes/no): yes

Pushing to GitHub...
Release complete! Tag v3.0.62007 has been pushed to GitHub.
```

### Troubleshooting

- **GPG signing fails**: Ensure `GPG_FINGERPRINT` is set correctly and your GPG key is available in your keyring
- **GitHub push fails**: Check your git credentials or SSH keys are configured correctly
- **goreleaser fails**: Ensure you have the latest version of goreleaser installed and check the `.goreleaser.yml` configuration

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

### CI Testing Strategy

Our CI workflow tests against multiple TiDB versions (latest of each minor series) to ensure compatibility across different releases. To optimize cache performance and avoid timeouts, we cache each TiDB version separately rather than caching all versions together. This approach:

- Prevents cache upload timeouts (each cache is ~700-800MB instead of 4.5GB)
- Allows each test job to download and cache only the version it needs
- Shares the TiUP binary cache across all tests for efficiency
- Automatically cleans up unused caches after 7 days

This strategy balances test coverage with CI performance and reliability.
