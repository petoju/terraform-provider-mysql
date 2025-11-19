# Plan: Keep TiDB Versions in Sync

## Problem

TiDB versions are currently defined in two places:
1. Test matrix: `testtidb6.1.0`, `testtidb6.5.3`, etc.
2. Pre-download step: `VERSIONS="6.1.0 6.5.3 7.1.5 7.5.2 8.1.0"`

This creates a maintenance burden - versions can get out of sync.

## Solution Options

### Option 1: Extract Versions from Matrix (Recommended)

Use GitHub Actions matrix variables to define versions once, then derive test targets.

**Pros:**
- Single source of truth
- Automatic sync
- Easy to add/remove versions

**Cons:**
- Requires restructuring the matrix

**Implementation:**
```yaml
tests:
  strategy:
    matrix:
      tidb_version:
        - "6.1.0"
        - "6.5.3"
        - "7.1.5"
        - "7.5.2"
        - "8.1.0"
      mysql_target:
        - testversion5.6
        - testversion5.7
        # ... other MySQL tests
      include:
        # Combine TiDB versions with test targets
        - tidb_version: "6.1.0"
          target: testtidb6.1.0
        - tidb_version: "6.5.3"
          target: testtidb6.5.3
        # ... etc
```

### Option 2: Use Environment Variable

Define versions as environment variable, use in both places.

**Pros:**
- Simple
- Single definition

**Cons:**
- Still requires manual updates
- Less flexible

**Implementation:**
```yaml
env:
  TIDB_VERSIONS: "6.1.0 6.5.3 7.1.5 7.5.2 8.1.0"

jobs:
  prepare-dependencies:
    steps:
      - name: Pre-download TiDB versions
        run: |
          for version in ${{ env.TIDB_VERSIONS }}; do
            tiup install tidb:v${version} || true
          done
```

### Option 3: Extract from Matrix Dynamically

Use GitHub Actions to extract versions from matrix and pass to prepare job.

**Pros:**
- Fully automatic
- No manual sync needed

**Cons:**
- More complex
- Requires job outputs

**Implementation:**
```yaml
prepare-dependencies:
  outputs:
    tidb_versions: ${{ steps.set-versions.outputs.versions }}
  steps:
    - name: Set TiDB versions
      id: set-versions
      run: |
        # Extract from matrix (would need to parse workflow file or use job outputs)
```

### Option 4: Use a Separate Job to Extract Versions

Create a job that reads the matrix and outputs versions.

**Pros:**
- Can parse matrix dynamically
- Single source of truth

**Cons:**
- Most complex
- Requires parsing YAML or using GitHub Actions features

## Recommended Solution: Option 2 (Environment Variable)

Simplest and most maintainable approach:

1. Define versions once as environment variable
2. Use in both test matrix (via script) and pre-download step
3. Easy to update - change one place

## Implementation

### Step 1: Define Versions at Workflow Level

```yaml
env:
  TIDB_VERSIONS: "6.1.0 6.5.3 7.1.5 7.5.2 8.1.0"
```

### Step 2: Use in Pre-download

```yaml
- name: Pre-download TiDB versions
  run: |
    export PATH=$HOME/.tiup/bin:$PATH
    for version in ${{ env.TIDB_VERSIONS }}; do
      echo "Pre-downloading TiDB components for v${version}..."
      tiup install tidb:v${version} || true
      tiup install pd:v${version} || true
      tiup install tikv:v${version} || true
    done
```

### Step 3: Generate Test Matrix from Versions

Use a script or GitHub Actions feature to generate test targets from versions.

However, GitHub Actions doesn't support dynamic matrix generation easily. So we'd need to:
- Keep matrix explicit but add comment referencing env var
- Or use a script to validate they match

### Alternative: Use Matrix Include Pattern

Better approach - define versions once, use matrix include:

```yaml
tests:
  strategy:
    matrix:
      tidb_version: ["6.1.0", "6.5.3", "7.1.5", "7.5.2", "8.1.0"]
      include:
        - tidb_version: "6.1.0"
          target: testtidb6.1.0
        - tidb_version: "6.5.3"
          target: testtidb6.5.3
        # ... etc
      exclude:
        - tidb_version: "6.1.0"  # Only run if target is set
```

But this doesn't help with pre-download.

## Best Practical Solution

**Use environment variable + validation script:**

1. Define versions as env var
2. Use in pre-download
3. Add validation step that checks matrix matches env var
4. Or use a script that generates both from a single source

Actually, the simplest is to:
- Keep versions in env var
- Use in pre-download
- Add a comment in matrix referencing the env var
- Add a validation step that fails if they don't match
