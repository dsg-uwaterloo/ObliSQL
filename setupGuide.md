# ObliSQL Complete Testing & Benchmark Guide

## Prerequisites
- Clone the Repository (if you haven't already)
    ```bash
    git clone https://github.com/A-karimKati/ObliSQL.git
    cd ObliSQL
    ```
- All dependencies installed (Docker, Go 1.23+, Thrift 0.22.0, Redis 8.0+, Ansible)
  


## Download Required Tracefiles
**CRITICAL:** ObliSQL requires tracefiles that are not in the GitHub repository.

**1. Download** tracefiles from: https://vault.cs.uwaterloo.ca/s/9CqsQTbsZdn832B


**2. Extract** the downloaded file to /tmp/oblisql_tracefiles
```bash
    mkdir -p /tmp/oblisql_tracefiles 
    tar -xvf OasisDB_DBFiles.tar -C /tmp/oblisql_tracefiles/

    cd /tmp/oblisql_tracefiles/OasisDB_DBFiles
    unzip Waffle.zip
    unzip ORAMFiles.zip
```

**3. Organize** tracefiles according to repository structure
```bash
cd ObliSQL dir  
mkdir -p ~/ObliSQL/waffle/tracefiles
cp -r /tmp/oblisql_tracefiles/OasisDB_DBFiles/DBTraceFiles/* ~/ObliSQL/waffle/tracefiles/

mkdir -p ~/ObliSQL/cmd/oramExecutor  
cp -r /tmp/oblisql_tracefiles/OasisDB_DBFiles/Maps/* ~/ObliSQL/cmd/oramExecutor/

mkdir -p /tmp/oblisql_redis_snapshots
cp -r /tmp/oblisql_tracefiles/OasisDB_DBFiles/EpinionsSnapshots /tmp/oblisql_redis_snapshots/
cp -r /tmp/oblisql_tracefiles/OasisDB_DBFiles/snapshot_only_rankings /tmp/oblisql_redis_snapshots/

# Verify key files are in place
ls -la ~/ObliSQL/waffle/tracefiles/serverInput.txt
ls -la ~/ObliSQL/cmd/oramExecutor/
```


4. **Build and Run** the image
```bash
docker build -t oblisql-complete .
docker run -it --network host --cap-add=NET_ADMIN oblisql-complete 
```





## Running ObliSQL Tests

### Quick Test (No Services Required)
```bash
cd /app
go test ./pkg/benchmark/queries_test.go
go test ./pkg/executorPlaintext/executorPlaintext_test.go  
go test ./pkg/oramExecutor/oram_test.go
```

### Full Resolver Test (Requires Running Services)

#### Step 1: Start All Services for Testing

**1. Start Redis:**
```bash
redis-server --daemonize yes
redis-cli ping  # Should return PONG
```

**2. Start Waffle Proxy Server:**
```bash
cd /app/waffle
./bin/proxy_server -l ./tracefiles/serverInput.txt -b 3000 -r 2000 -f 170 -d 2000 -c 2 -n 4 -h 127.0.0.1 -p 6379 &
```

**Wait for (takes ~25 seconds):**
```
Successfully initialized waffle with keys size 6095882
Initialized Waffle
Proxy server is reachable
Thrift: TNonblockingServer: IO thread #0 entering loop...
```

**3. Start BatchManager:**
```bash
cd /app/cmd
./batchManager/batchManager -p 9500 -R 2000 -Z 500 -num 1 -T Waffle -X 2 -hosts 127.0.0.1 -ports 9090 &
```

**Wait for:**
```
"Connected to Executors!"
"Launching Central Coordinator with timeOut: 500"
```

**4. Start Resolver (for testing use port 9900):**
```bash
cd /app/cmd/resolver  
./resolver -p 9900 -bh 127.0.0.1 -bp 9500 &
```

**Wait for:**
```
"Resolver Connected!"
"Connected to load balancer at 127.0.0.1:9500"
```

#### Step 2: Run the Resolver Test
```bash
cd /app
go test ./pkg/resolver/resolver_test.go
```

**Expected output:**
```
ok      command-line-arguments  25.536s
```



## Running ObliSQL Benchmarks

### Step 1: Start All Services for Benchmarking

**Follow the same steps as testing above, but start resolver on port 9600:**

**1-3. Same as testing steps 1-3**

**4. Start Resolver (for benchmarks use port 9600):**
```bash
cd /app/cmd/resolver
./resolver -p 9600 -bh 127.0.0.1 -bp 9500 &
```

### Step 2: Verify All Services Are Running
```bash
jobs
```

**Should show:**
```
[1] Running ./bin/proxy_server ... (wd: /app/waffle)
[2] Running ./batchManager/batchManager ... (wd: /app/cmd)  
[3] Running ./resolver ... (wd: /app/cmd/resolver)
```

### Step 3: Run Benchmarks

#### Basic Scaling Benchmark (from README)
```bash
cd /app/pkg/benchmark
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 1000 -t 30 -q scaling
```

#### Other Available Benchmarks

**Join Query Benchmark:**
```bash
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 1000 -t 30 -jr 1
```

**Range Query Benchmark:**
```bash
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 1000 -t 30 -jr 2 -rs 10
```

**Different Concurrent Request Loads:**
```bash
# Light load
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 500 -t 30 -q scaling

# Medium load  
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 1000 -t 30 -q scaling

# Heavy load
../../cmd/benchmark/benchmark -h 127.0.0.1 -p 9600 -s 2000 -t 30 -q scaling
```

---

## Test vs Benchmark Port Differences

| Component | Testing Port | Benchmark Port | Notes |
|-----------|-------------|----------------|-------|
| Resolver | 9900 | 9600 | Tests expect 9900, benchmarks expect 9600 |
| BatchManager | 9500 | 9500 | Same for both |
| Proxy Server | 9090 | 9090 | Same for both |

## Expected Results

### Test Results
```bash
# Successful test output:
ok      command-line-arguments  25.536s
```

### Benchmark Results
```bash
# Successful benchmark includes:
Client Connected!
Connected to Resolver 127.0.0.1:9600
Query Type: scaling
[Performance metrics and statistics]
```

## Complete Service Restart Procedure

**If you need to restart everything:**
```bash
# 1. Kill all background jobs
kill %1 %2 %3 2>/dev/null || true

# 2. Check nothing is running
jobs

# 3. Restart Redis if needed
redis-server --daemonize yes

# 4. Follow steps 2-4 from either testing or benchmark sections
```

## Benchmark Parameters Explained

- `-h 127.0.0.1` - Resolver host address
- `-p 9600` - Resolver port (9900 for tests, 9600 for benchmarks)
- `-s 1000` - Number of in-flight (concurrent) requests
- `-t 30` - Benchmark duration in seconds
- `-q scaling` - Query type (scaling, select, etc.)
- `-jr 1` - Join query type (1=Join, 2=Range)
- `-rs 10` - Range size for range queries

## Troubleshooting

**If services fail to start:**
1. Check that Redis is running: `redis-cli ping`
2. Wait for each service to fully initialize before starting the next
3. Verify ports aren't in use: `netstat -tlnp | grep -E '(6379|9090|9500|9600|9900)'`

**If tests/benchmarks fail:**
1. Ensure all 3 services are running: `jobs`
2. Check you're using the correct resolver port (9900 for tests, 9600 for benchmarks)
3. For benchmarks, ensure you're in `/app/pkg/benchmark` directory
4. Restart services if timeouts occur

## Performance Notes

- Proxy server initialization takes ~25 seconds (loading 6M+ keys)
- BatchManager may timeout if idle - restart if needed
- Test results depend on system resources and Docker configuration
- Use different `-s` values to test various load levels