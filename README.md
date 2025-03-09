**NOTE**: Although this system is intended & recommended to run on different machines for scale factors greater than 2, we can still run all processes on the same machine. 


### Requirements:
* Ansible 
* Cmake-3.5+
* Thrift RPC library (https://thrift.apache.org/)
* Go >= 1.23
* Redis-Server >= 7.0 (https://redis.io/docs/getting-started/installation/install-redis-on-linux/)


## Installation 

1. Clone the repository onto your machine(s).
2. Download the DB Tracefiles available [here](https://vault.cs.uwaterloo.ca/s/9CqsQTbsZdn832B). 


### Building Waffle:
1. Go into the waffle folder.

```
sh build.sh
```

2. You should be able to execute waffle/bin/proxy_server

### Building Layers and ORAM 

1. Go into the cmd folder.

```
chmod +x ./build.sh && build.sh
```

2. You should be see all executables populate in each of the indivdual folders. 


## Running

We provide ansible playbooks to run processes. 


### Running Epinions/Big Data Benchmark with Waffle

1. Download Waffle tracefile folder from the provided link. (These are not uploaded to github due to new file size restrictions)
2. Create tracefiles folder in /waffle/
3. Move unzipped tracefiles here. 
4. Go to /AnsibleScripts/WaffleData/ 
5. Each folder here contains a `deploy.yml` and `inventory.ini` file. 
6. You can adjust the `inventory.ini` for your machines (Ip addresses or aliases). **Note**: If you are running multiple processes on the same machine, please make sure to port number do not conflict. 
7. Make sure `redis-server` is running on the available port you specify in `inventory.ini`

Run: 
```
sh bench.sh --s-values <in-flight request list>

(Example: sh bench.sh --s-values 1000,2000,3000)
```

8. This will sequentially run benchmarks for the specified in-flight requests (Concurrent requests)
9. After completion of each benchmark, a new file will be generated containing total # of requests performed in 30 seconds along with average latency
10. By default we use the epinions dataset. To run the Big Data Benchmark, please pass `-bdb` to the resolver process and `-q bdb` to the benchmark process. This is already done in all ansible files provided for big data benchmark. 


### Running Epinions/Big Data Benchmark with Oram
*Note:* Due to high overhead incurred by ORAM, we provide redis-snapshots that contain the correct dataset and position map. This improves loading time from hours to a couple of minutes. 

1. Download ORAM tracefiles from the provided link.
2. Copy content from `Maps` to /cmd/oramExecutor
3. Copy `EpinionsSnapshots` and `snapshot_rankings` to a folder on the machine that will have redis running on it. 
4. Specify the location of these folders under the oramDeploy.yml playbooks. Redis should be able to load these tracefiles directly. 
5. Follow instructions `6 - 10` specified above. 


### Running Tests

We provide a tests that cover various queries. These are available under `/pkg/resolver/resolver_tests.go`. We provide instructions to run these tests on Scale-1. These can be extended to further scales as needed. 

1. Run the key/value store you want. For this tutorial we'll use waffle. 

```
./bin/proxy_server  -l <TRACE_FILE> -b <BATCH_SIZE> -r <SYSTEM_PARAMETER> -f <FAKE_QUERIES_FOR_DUMMY_OBJECTS> -d <NUMBER_OF_DUMMY_OBJECTS> -c <CACHE_SIZE> -n <NUM_CORES> -h <STORAGE_SERVER_IP> -p <STORAGE_PORT>

(Example: ./bin/proxy_server -l ./tracefiles/serverInput.txt -b 3000 -r 2000 -f 170 -d 2000 -c 2 -n 4 -h 127.0.0.1 -p 6379)
```

tracefile would point to `serverInput.txt` under tracefiles. 

2. Run the Batcher:

```
./cmd/batchManager/batchManager -p <Batcher_Port> -R <Batch_Size> -Z <Timeout> -num <Scale_Factor> -T <Type> -X <Batch Workers> -hosts <KV Hosts> -ports <KV Ports>

(Example: ./cmd/batchManager/batchManager -p 9500 -R 2000 -Z 500 -num 1 -T Waffle -X 2 -hosts 127.0.0.1 -ports 9090)
```

3. Run the Resolver: 

```
./cmd/resolver/resolver -p <Resolver_Port> -bh <Batcher_Host(s)> -bp <Batcher_Ports> 

(Example: ./cmd/resolver/resolver -p 9600 -bh 127.0.0.1 -bp 9500)
```

You can optionally add `-bf` to enable bloom filters. You can add `-bf -jo` to enable hybrid bloom filter joins. 

4. Running Benchmarks/Tests

To Run Tests: 

```
go test ./pkg/resolver/resolver_tests.go
```

To run benchmark: 

```
./cmd/benchmark/benchmark -h <Resolver_Host(s)> -p <Resolver_Port(s)> -s <In-flight requests> -t <Benchmark Time> -q <Query Type>

(Example: ./cmd/benchmark/benchmark -h 127.0.01 -p 9600 -s 1000 -t 30 -q scaling)
```

You can also just run Join and Range bloom by passing `- jr <TYPE>` where `Type=1` is for Join Query and `Type=2` is for Range. 
If you are running `Type=2` then also pass range size `-rs <SIZE>`. It will default to 5.



### DB Trace Files: 
https://vault.cs.uwaterloo.ca/s/9CqsQTbsZdn832B
