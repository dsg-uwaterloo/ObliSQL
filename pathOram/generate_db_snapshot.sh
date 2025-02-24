#!/bin/bash

# Usage message if no action is provided.
if [ $# -lt 1 ]; then
  echo "Usage: $0 {gen|test} [logCapacity] [tracefile] [batchSize]"
  exit 1
fi

ACTION=$1

# Default values for generate parameters.
DEFAULT_LOG_CAPACITY=20
DEFAULT_TRACEFILE="/Users/nachiketrao/Desktop/URA/Tracefiles/serverInput512.txt"
DEFAULT_BATCH_SIZE=500

if [ "$ACTION" == "gen" ]; then
  # Use positional parameters if provided, else use defaults.
  LOG_CAPACITY=${2:-$DEFAULT_LOG_CAPACITY}
  TRACEFILE=${3:-$DEFAULT_TRACEFILE}
  BATCH_SIZE=${4:-$DEFAULT_BATCH_SIZE}

  echo "Generating db snapshot with logCapacity: $LOG_CAPACITY, tracefile: $TRACEFILE, batchSize: $BATCH_SIZE"
  go run ./tests/generate_db_snapshot.go -logCapacity=$LOG_CAPACITY -tracefile="$TRACEFILE" -batchSize=$BATCH_SIZE

  sleep 120
  mv ./dump.rdb ./snapshot_redis.rdbb # Save file differently to avoid redis interference
  if [ $? -ne 0 ]; then
    echo "Generating db snapshot failed."
    exit 1
  fi

elif [ "$ACTION" == "test" ]; then
  echo "Testing db snapshot..."
  go run ./tests/test_db_snapshot.go
  if [ $? -ne 0 ]; then
    echo "Testing db snapshot failed."
    exit 1
  fi

else
  echo "Invalid flag. Use 'gen' for generate or 'test' for test."
  exit 1
fi

echo "Execution completed successfully."
