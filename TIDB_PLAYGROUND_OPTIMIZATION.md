# TiDB Test Optimization: Switch to TiDB Playground

## Current Problem

TiDB tests are slow because they:
1. **Pull 3 large Docker images** (PD, TiKV, TiDB) - ~500MB-1GB total
2. **Start 3 containers sequentially** (PD → TiKV → TiDB)
3. **Wait for cluster coordination** - PD and TiKV need to sync before TiDB can start
4. **No image caching** - Images are pulled fresh each time

**Current timing**: TiDB tests take 5-10+ minutes vs 1-2 minutes for regular MySQL tests

## Solution: Use TiDB Playground

TiDB Playground is TiDB's official tool for quickly spinning up local TiDB clusters. It's optimized for testing and development.

### Benefits:
- **Single command** to start entire cluster
- **Pre-configured** - no manual coordination needed
- **Faster startup** - optimized for local use
- **Single Docker image** option (all-in-one) or binary
- **Better caching** - can use pre-pulled images

## Implementation Options

### Option 1: TiUP Playground (Recommended)

TiUP is TiDB's package manager and includes `tiup playground` command.

**Pros:**
- Official TiDB tool
- Supports all TiDB versions
- Fast startup (~30-60 seconds)
- Can specify exact version
- Single command

**Cons:**
- Requires installing TiUP first
- Binary download (~10-20MB)

**Installation:**
```bash
curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
```

**Usage:**
```bash
tiup playground v7.5.2 --db 1 --kv 1 --pd 1 --tiflash 0 --without-monitor
```

### Option 2: Docker Compose with Pre-pulled Images

Use docker-compose with image caching.

**Pros:**
- Uses existing Docker infrastructure
- Can cache images between runs
- More control

**Cons:**
- Still slower than playground
- More complex setup

### Option 3: TiDB Playground Docker Image

Use the official `pingcap/tidb-playground` Docker image.

**Pros:**
- Single Docker image
- Pre-configured
- Fast startup

**Cons:**
- May not support all versions
- Less control over components

## Recommended Implementation: TiUP Playground

### Step 1: Install TiUP in GitHub Actions

Add to workflow:
```yaml
- name: Install TiUP
  run: |
    curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
    export PATH=$HOME/.tiup/bin:$PATH
    tiup --version
```

### Step 2: Update Makefile

Replace the current `testtidb` target:

```makefile
testtidb:
	@export PATH=$$HOME/.tiup/bin:$$PATH && \
	tiup playground $(MYSQL_VERSION) --db 1 --kv 1 --pd 1 --tiflash 0 --without-monitor --host 0.0.0.0 --db.port $(MYSQL_PORT) & \
	PLAYGROUND_PID=$$! && \
	echo "Waiting for TiDB..." && \
	while ! mysql -h 127.0.0.1 -P $(MYSQL_PORT) -u "$(TEST_USER)" -e 'SELECT 1' >/dev/null 2>&1; do \
		printf '.'; sleep 1; \
	done; \
	echo "Connected!" && \
	MYSQL_USERNAME="$(TEST_USER)" MYSQL_PASSWORD="" MYSQL_ENDPOINT=127.0.0.1:$(MYSQL_PORT) $(MAKE) testacc; \
	TEST_RESULT=$$?; \
	kill $$PLAYGROUND_PID || true; \
	exit $$TEST_RESULT
```

### Step 3: Alternative - Simpler Script Approach

Create a new script `scripts/tidb-playground.sh`:

```bash
#!/usr/bin/env bash
set -e

VERSION=${1:-7.5.2}
PORT=${2:-4000}
MODE=${3:-start}  # start or stop

export PATH=$HOME/.tiup/bin:$PATH

if [ "$MODE" = "start" ]; then
    echo "Starting TiDB Playground v${VERSION} on port ${PORT}..."
    tiup playground ${VERSION} \
        --db 1 \
        --kv 1 \
        --pd 1 \
        --tiflash 0 \
        --without-monitor \
        --host 0.0.0.0 \
        --db.port ${PORT} \
        --db.config "" \
        &
    PLAYGROUND_PID=$!
    echo $PLAYGROUND_PID > /tmp/tidb-playground-${PORT}.pid
    
    # Wait for TiDB to be ready
    echo "Waiting for TiDB..."
    for i in {1..60}; do
        if mysql -h 127.0.0.1 -P ${PORT} -u root -e 'SELECT 1' >/dev/null 2>&1; then
            echo "TiDB is ready!"
            exit 0
        fi
        sleep 1
    done
    echo "TiDB failed to start"
    exit 1
elif [ "$MODE" = "stop" ]; then
    if [ -f /tmp/tidb-playground-${PORT}.pid ]; then
        PID=$(cat /tmp/tidb-playground-${PORT}.pid)
        kill $PID 2>/dev/null || true
        rm /tmp/tidb-playground-${PORT}.pid
    fi
    # Also kill any remaining tiup processes
    pkill -f "tiup playground" || true
fi
```

Then update Makefile:
```makefile
testtidb:
	@export PATH=$$HOME/.tiup/bin:$$PATH && \
	$(CURDIR)/scripts/tidb-playground.sh $(MYSQL_VERSION) $(MYSQL_PORT) start && \
	MYSQL_USERNAME="$(TEST_USER)" MYSQL_PASSWORD="" MYSQL_ENDPOINT=127.0.0.1:$(MYSQL_PORT) $(MAKE) testacc; \
	TEST_RESULT=$$?; \
	$(CURDIR)/scripts/tidb-playground.sh $(MYSQL_VERSION) $(MYSQL_PORT) stop; \
	exit $$TEST_RESULT
```

## Expected Performance Improvement

**Current**: 5-10+ minutes per TiDB test
- Docker image pulls: 2-5 minutes
- Container startup: 1-2 minutes  
- Cluster coordination: 1-2 minutes
- Test execution: 1-2 minutes

**With TiUP Playground**: 2-3 minutes per TiDB test
- TiUP install (cached): ~5 seconds
- Playground startup: 30-60 seconds
- Test execution: 1-2 minutes

**Savings**: ~3-7 minutes per TiDB test × 6 TiDB tests = **18-42 minutes** per workflow run!

## Implementation Checklist

- [ ] Install TiUP in GitHub Actions workflow
- [ ] Create tidb-playground.sh script (or update existing)
- [ ] Update Makefile testtidb target
- [ ] Test locally with TiUP
- [ ] Test in GitHub Actions
- [ ] Remove old tidb-test-cluster.sh script (or keep as fallback)

## References

- TiUP Documentation: https://docs.pingcap.com/tidb/stable/tiup-overview
- TiUP Playground: https://docs.pingcap.com/tidb/stable/tiup-playground
- TiUP Installation: https://docs.pingcap.com/tidb/stable/tiup-overview#install-tiup
