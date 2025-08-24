This is an enhanced implementation of Path ORAM.

To run local Redis server: ./start_redis_serve.sh

To run the tests: ./run_test_suite.sh

To run main: ./makerun.sh


## How to do snapshot:
In generate_db_snapshot.go, change

const (
	logCapacity = 10 // Logarithm base 2 of capacity (1024 buckets)
	Z           = 4  // Number of blocks per bucket
	stashSize   = 20 // Maximum number of blocks in stash
)

Make sure to place the tracefile and define the path in `ORAM_SNAPSHOT/pathOram/tests/generate_db_snapshot.go`

The generated snapshot in : dump.rdb and proxy_snapshot.json in the root of this directory

## How to use snapshot:
Set correct value for arguments logCapacity, Z, stashSize, set the -snapshot flag, tracefile path.

- Make sure that the generate rdb and executor oram code use the same Key with input `oblisqloram`
