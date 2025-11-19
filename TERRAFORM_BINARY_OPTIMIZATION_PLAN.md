# Plan: Optimize Terraform Binary Build in GitHub Actions

## Problem Statement

Currently, each integration test job in the GitHub Actions matrix independently:
1. Downloads and extracts the Terraform binary (version 1.5.6)
2. Downloads all Go module dependencies (~90+ packages, ~100-200MB)

This results in:
- **Terraform**: Redundant downloads (13 test jobs × ~20MB = ~260MB downloaded repeatedly)
- **Go Modules**: Redundant downloads (13 test jobs × ~100-200MB = ~1.3-2.6GB downloaded repeatedly!)
- Increased test execution time (each job waits for downloads/extraction)
- Unnecessary network usage and costs

## Current Architecture

### Current Flow:
```
Each test job (testversion5.6, testversion8.0, etc.):
  1. Checkout repo
  2. Install mysql-client
  3. Run `make testversion5.6` (or other target)
     → `make testacc` (dependency)
       → `make bin/terraform` (downloads & extracts Terraform)
     → Run actual tests
```

### Makefile Dependencies:
- `testacc` depends on `bin/terraform`
- `bin/terraform` target downloads Terraform 1.5.6 for Linux amd64
- Binary is placed in `$(CURDIR)/bin/terraform`
- Tests use `PATH="$(CURDIR)/bin:${PATH}"` to find terraform

## Solution: Pre-build Job with Artifact Sharing

### Proposed Architecture:
```
1. prepare-terraform job:
   - Downloads Terraform binary once
   - Uploads as GitHub Actions artifact
   
2. Each test job:
   - Downloads Terraform artifact
   - Extracts to bin/
   - Runs tests (no download needed)
```

## Implementation Plan

### Step 1: Create `prepare-terraform` Job

**New job that runs before tests:**

```yaml
prepare-terraform:
  name: Prepare Terraform Binary
  runs-on: ubuntu-22.04
  steps:
    - name: Checkout Git repo
      uses: actions/checkout@v4
    
    - name: Set up Go (for go mod download if needed)
      uses: actions/setup-go@v4
      with:
        go-version-file: go.mod
    
    - name: Download Terraform
      run: |
        mkdir -p bin
        curl -sfL https://releases.hashicorp.com/terraform/1.5.6/terraform_1.5.6_linux_amd64.zip > bin/terraform.zip
        cd bin && unzip terraform.zip && rm terraform.zip
        chmod +x bin/terraform
    
    - name: Upload Terraform binary
      uses: actions/upload-artifact@v4
      with:
        name: terraform-binary
        path: bin/terraform
        retention-days: 1
```

### Step 2: Update `tests` Job to Download Artifact and Cache Go Modules

**Modify existing tests job:**

```yaml
tests:
  runs-on: ubuntu-22.04
  needs: [prepare-terraform]  # Add dependency
  strategy:
    fail-fast: false
    matrix:
      target:
        - testversion5.6
        - testversion5.7
        # ... rest of targets
  steps:
    - name: Checkout Git repo
      uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version-file: go.mod
    
    - name: Cache Go modules
      uses: actions/cache@v4
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
    
    - name: Download Terraform binary
      uses: actions/download-artifact@v4
      with:
        name: terraform-binary
        path: bin/
    
    - name: Make Terraform executable
      run: chmod +x bin/terraform
    
    - name: Install mysql client
      run: |
        sudo apt-get update
        sudo apt-get -f -y install mysql-client
    
    - name: Run tests {{ matrix.target }}
      run: make ${{ matrix.target }}
```

### Step 3: Handle Makefile Compatibility

**The Makefile already supports this!**

The `bin/terraform` target in the Makefile:
```makefile
bin/terraform:
	mkdir -p "$(CURDIR)/bin"
	curl -sfL https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(TERRAFORM_OS)_$(ARCH).zip > $(CURDIR)/bin/terraform.zip
	(cd $(CURDIR)/bin/ ; unzip terraform.zip)
```

**Solution Options:**

**Option A: Create bin/terraform before running make (Recommended)**
- Download artifact to `bin/terraform` before running make
- Make will see the file exists and skip the target
- No Makefile changes needed

**Option B: Modify Makefile to check for existing binary**
- Add check: `if [ -f "$(CURDIR)/bin/terraform" ]; then exit 0; fi`
- More robust but requires Makefile change

**Option C: Use Makefile variable to skip download**
- Add `SKIP_TERRAFORM_DOWNLOAD=true` environment variable
- Modify Makefile to check this variable
- More explicit but requires Makefile change

**Recommended: Option A** - Simplest, no Makefile changes needed.

### Step 4: Update Makefile (Optional Enhancement)

**If we want to make it more explicit, add a check:**

```makefile
bin/terraform:
	@if [ -f "$(CURDIR)/bin/terraform" ]; then \
		echo "Terraform binary already exists, skipping download"; \
		exit 0; \
	fi
	mkdir -p "$(CURDIR)/bin"
	curl -sfL https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(TERRAFORM_OS)_$(ARCH).zip > $(CURDIR)/bin/terraform.zip
	(cd $(CURDIR)/bin/ ; unzip terraform.zip)
```

This makes the Makefile idempotent and works for both CI and local development.

## Benefits

### Terraform Binary Optimization:
1. **Time Savings**: 
   - Current: ~5-10 seconds per job × 13 jobs = 65-130 seconds
   - Optimized: ~5-10 seconds once + ~1 second download per job = ~18 seconds total
   - **Savings: ~50-110 seconds per workflow run**

2. **Network Savings**:
   - Current: ~260MB downloaded (13 × ~20MB)
   - Optimized: ~20MB downloaded once + ~20MB uploaded once
   - **Savings: ~220MB per workflow run**

### Go Module Caching Optimization:
3. **Time Savings**:
   - Current: ~10-30 seconds per job × 13 jobs = 130-390 seconds
   - Optimized: First run downloads (~10-30s), subsequent runs use cache (~1-2s)
   - **Savings: ~120-360 seconds per workflow run** (after first run)

4. **Network Savings**:
   - Current: ~1.3-2.6GB downloaded (13 × ~100-200MB)
   - Optimized: ~100-200MB downloaded once, cached for all jobs
   - **Savings: ~1.1-2.4GB per workflow run**

### Combined Total Savings:
- **Time**: ~170-470 seconds (~3-8 minutes) per workflow run
- **Network**: ~1.3-2.6GB per workflow run

3. **Reliability**:
   - Single point of download reduces chance of network failures
   - Artifact caching can further improve reliability

4. **Cost**:
   - Reduced network egress costs
   - Faster test execution = lower compute costs

## Implementation Details

### Artifact Considerations

**Artifact Size**: Terraform binary is ~20MB (compressed), ~50MB uncompressed

**Retention**: Set to 1 day (artifacts are only needed during workflow execution)

**Artifact Name**: `terraform-binary` (unique per workflow run)

### Parallel Execution

The `prepare-terraform` job will run first, then all test jobs run in parallel. This is optimal because:
- Tests can't run without Terraform anyway
- Parallel test execution is maintained
- Minimal impact on total workflow time

### Error Handling

If `prepare-terraform` fails:
- All dependent test jobs will be skipped (GitHub Actions behavior)
- Clear error message in workflow UI
- Easy to identify the root cause

### Fallback Behavior

If artifact download fails in a test job:
- The Makefile will still attempt to download (if Option A)
- Or we can add explicit error handling in the workflow

## Testing Strategy

1. **Test locally first**:
   - Manually download Terraform to `bin/terraform`
   - Run `make testversion8.0` to verify it works

2. **Test in GitHub Actions**:
   - Create a test branch
   - Run workflow and verify:
     - `prepare-terraform` completes successfully
     - Artifact is uploaded
     - Test jobs download artifact
     - Tests run successfully
     - No redundant downloads occur

3. **Verify performance**:
   - Compare workflow execution times before/after
   - Check artifact sizes
   - Monitor for any failures

## Alternative Approaches Considered

### Option 1: Use GitHub Actions Cache for Terraform
```yaml
- uses: actions/cache@v4
  with:
    path: bin/terraform
    key: terraform-1.5.6-linux-amd64
```
**Pros**: Automatic caching, no separate job needed
**Cons**: Cache miss still requires download, less explicit control
**Note**: This is actually what we're doing for Go modules (better fit)

### Option 2: Pre-download Go Modules in prepare Job
**Pros**: Could pre-populate cache
**Cons**: Not necessary - Go's built-in caching + actions/cache is sufficient

### Option 3: Docker Image with Pre-installed Terraform
**Pros**: Faster startup, includes all dependencies
**Cons**: Requires maintaining custom Docker image, more complexity

### Option 4: Use actions/setup-terraform
**Pros**: Official action, well-maintained
**Cons**: May not support Terraform 1.5.6 (older version), less control

## Recommended Implementation

**Use the artifact approach** because:
1. Explicit and predictable
2. Works with existing Makefile without changes
3. Easy to debug and maintain
4. Good performance characteristics
5. No external dependencies

## Implementation Checklist

- [ ] Create `prepare-terraform` job in workflow
- [ ] Add `needs: [prepare-terraform]` to tests job
- [ ] Add `actions/setup-go@v4` step to tests job
- [ ] Add `actions/cache@v4` step for Go modules to tests job
- [ ] Add artifact download step to tests job
- [ ] (Optional) Add idempotency check to Makefile `bin/terraform` target
- [ ] Test locally with pre-downloaded binary
- [ ] Test in GitHub Actions on a test branch
- [ ] Monitor workflow execution times (should see significant improvement)
- [ ] Monitor cache hit rates in GitHub Actions
- [ ] Document the optimization in README (optional)

## Code Changes Summary

### `.github/workflows/main.yml`

**Add new job:**
```yaml
prepare-terraform:
  name: Prepare Terraform Binary
  runs-on: ubuntu-22.04
  steps:
    - name: Checkout Git repo
      uses: actions/checkout@v4
    - name: Download Terraform
      run: |
        mkdir -p bin
        curl -sfL https://releases.hashicorp.com/terraform/1.5.6/terraform_1.5.6_linux_amd64.zip > bin/terraform.zip
        cd bin && unzip terraform.zip && rm terraform.zip
        chmod +x bin/terraform
    - name: Upload Terraform binary
      uses: actions/upload-artifact@v4
      with:
        name: terraform-binary
        path: bin/terraform
        retention-days: 1
```

**Modify tests job:**
```yaml
tests:
  needs: [prepare-terraform]  # Add this line
  # ... rest of job config
  steps:
    - name: Checkout Git repo
      uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version-file: go.mod
    
    - name: Cache Go modules
      uses: actions/cache@v4
      with:
        path: |
          ~/.cache/go-build
          ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        restore-keys: |
          ${{ runner.os }}-go-
    
    - name: Download Terraform binary
      uses: actions/download-artifact@v4
      with:
        name: terraform-binary
        path: bin/
    
    - name: Make Terraform executable
      run: chmod +x bin/terraform
    # ... rest of steps
```

### `GNUmakefile` (Optional Enhancement)

```makefile
bin/terraform:
	@if [ -f "$(CURDIR)/bin/terraform" ]; then \
		echo "Terraform binary already exists, skipping download"; \
		exit 0; \
	fi
	mkdir -p "$(CURDIR)/bin"
	curl -sfL https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(TERRAFORM_OS)_$(ARCH).zip > $(CURDIR)/bin/terraform.zip
	(cd $(CURDIR)/bin/ ; unzip terraform.zip)
```

## References

- [GitHub Actions Artifacts Documentation](https://docs.github.com/en/actions/using-workflows/storing-workflow-data-as-artifacts)
- [GitHub Actions Job Dependencies](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idneeds)
- [actions/upload-artifact@v4](https://github.com/actions/upload-artifact)
- [actions/download-artifact@v4](https://github.com/actions/download-artifact)
