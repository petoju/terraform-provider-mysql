# GitHub Actions Workflow Optimization Analysis

## Current Performance Analysis

Based on run https://github.com/zph/terraform-provider-mysql/actions/runs/19516178533:

### Job Timings Summary

**Prepare Dependencies**: ~22s
- Set up Go: ~10s
- Vendor Go dependencies: ~1s  
- Download Terraform: ~0s
- Upload artifacts: ~1-2s

**Test Jobs** (13 jobs, running in parallel): 76-105s each
- Set up job: 1-3s
- Checkout Git repo: 0-1s
- **Set up Go: 9-13s** ⚠️ (repeated 13 times!)
- Download Terraform binary: 1-3s
- Download vendor directory: 1-2s
- **Install mysql client: ~5-10s** ⚠️ (apt-get update is slow)
- Run tests: 60-90s (actual test execution)

**Total workflow time**: ~2-3 minutes (determined by longest test job)

## Optimization Opportunities

### 1. ⚠️ HIGH IMPACT: Optimize mysql-client Installation

**Current**: Each job runs `sudo apt-get update` which is slow (5-10s)

**Solution**: Skip apt-get update or use cached packages

```yaml
- name: Install mysql client
  run: |
    sudo apt-get update -qq  # -qq makes it quieter and slightly faster
    sudo apt-get install -y --no-install-recommends mysql-client
```

**Better Solution**: Use apt cache or skip update if possible

```yaml
- name: Install mysql client
  run: |
    sudo apt-get install -y --no-install-recommends mysql-client || \
    (sudo apt-get update -qq && sudo apt-get install -y --no-install-recommends mysql-client)
```

**Best Solution**: Cache apt packages

```yaml
- name: Cache apt packages
  uses: actions/cache@v4
  with:
    path: /var/cache/apt
    key: ${{ runner.os }}-apt-${{ hashFiles('**/.github/workflows/main.yml') }}
    restore-keys: |
      ${{ runner.os }}-apt-

- name: Install mysql client
  run: |
    sudo apt-get update -qq
    sudo apt-get install -y --no-install-recommends mysql-client
```

**Expected Savings**: 3-5 seconds per job × 13 jobs = **39-65 seconds**

### 2. ⚠️ MEDIUM IMPACT: Optimize Go Setup

**Current**: `actions/setup-go@v4` takes 9-13s per job

**Issue**: Go is being set up 13 times independently

**Solution**: This is somewhat unavoidable, but we can:
- Ensure we're using the latest version of the action (should be faster)
- Consider caching Go installation (though actions/setup-go should handle this)

**Note**: `actions/setup-go` should cache Go installations, so subsequent runs might be faster. The 9-13s might be acceptable.

**Potential Savings**: Minimal (this is likely already optimized by GitHub Actions)

### 3. ⚠️ MEDIUM IMPACT: Optimize Artifact Downloads

**Current**: 1-3s per artifact download

**Optimization**: Use parallel downloads or optimize compression

The vendor directory upload already uses `compression-level: 6` which is good. We could:
- Use `compression-level: 9` for better compression (smaller downloads, but slower upload)
- Or use `compression-level: 1` for faster uploads (larger downloads, but faster overall)

**Expected Savings**: 1-2 seconds per job × 13 jobs = **13-26 seconds**

### 4. ⚠️ LOW-MEDIUM IMPACT: Optimize Test Execution

**Current**: Tests take 60-90s per job

**Potential Optimizations**:
- Run tests in parallel within each job (if not already)
- Optimize test timeouts
- Skip unnecessary test setup/teardown

**Note**: This requires code changes and might affect test reliability.

### 5. ⚠️ LOW IMPACT: Optimize Checkout

**Current**: Checkout takes 0-1s

**Optimization**: Use `actions/checkout@v4` with `fetch-depth: 0` only if needed, or use `fetch-depth: 1` for faster checkout

```yaml
- name: Checkout Git repo
  uses: actions/checkout@v4
  with:
    fetch-depth: 1  # Only fetch latest commit, faster
```

**Expected Savings**: Minimal (~0.5s per job)

## Recommended Implementation Priority

### Priority 1: Optimize mysql-client Installation (HIGHEST IMPACT)
- **Effort**: Low
- **Impact**: High (39-65 seconds saved)
- **Risk**: Low

### Priority 2: Optimize Artifact Compression
- **Effort**: Low  
- **Impact**: Medium (13-26 seconds saved)
- **Risk**: Low

### Priority 3: Optimize Checkout Depth
- **Effort**: Low
- **Impact**: Low (~6 seconds saved)
- **Risk**: Very Low

## Implementation Plan

### Step 1: Add apt package caching

```yaml
- name: Cache apt packages
  uses: actions/cache@v4
  with:
    path: /var/cache/apt
    key: ${{ runner.os }}-apt-${{ hashFiles('**/.github/workflows/main.yml') }}
    restore-keys: |
      ${{ runner.os }}-apt-

- name: Install mysql client
  run: |
    sudo apt-get update -qq
    sudo apt-get install -y --no-install-recommends mysql-client
```

### Step 2: Optimize checkout (if changelog generation not needed)

```yaml
- name: Checkout Git repo
  uses: actions/checkout@v4
  with:
    fetch-depth: 1  # Only if changelog generation not needed
```

### Step 3: Consider artifact compression tuning

Current compression-level: 6 is a good balance. Could experiment with:
- Level 1: Faster upload, larger download
- Level 9: Slower upload, smaller download

## Expected Total Savings

- **Apt caching**: 39-65 seconds
- **Artifact optimization**: 13-26 seconds  
- **Checkout optimization**: ~6 seconds
- **Total**: ~58-97 seconds (~1-1.5 minutes) per workflow run

## Additional Considerations

### Docker-based Approach (Future Consideration)

For even better performance, consider using a custom Docker image with:
- Go pre-installed
- mysql-client pre-installed
- All dependencies cached

This would eliminate:
- Go setup time (9-13s × 13 = 117-169s)
- mysql-client installation time (5-10s × 13 = 65-130s)

**Total potential savings**: ~3-5 minutes per workflow run

However, this requires:
- Maintaining a Docker image
- Building and pushing the image
- More complex setup

**Recommendation**: Start with apt caching first, then consider Docker if more optimization is needed.
