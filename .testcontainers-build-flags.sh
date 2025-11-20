#!/bin/bash
# Build flags to suppress warnings from testcontainers dependencies
# Usage: source .testcontainers-build-flags.sh before building

export CGO_CFLAGS="-Wno-gnu-folding-constant"
