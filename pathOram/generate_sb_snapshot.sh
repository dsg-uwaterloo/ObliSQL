#!/bin/bash

# Check if a flag is provided
if [ -z "$1" ]; then
  echo "Usage: $0 {gen|test}"
  exit 1
fi

# Run based on the flag provided
if [ "$1" == "gen" ]; then
  echo "Generating db snapshot..."
  go run ./tests/generate_db_snapshot.go
  mv ./dump.rdb ./snapshot_redis.rdbb # Save file differently to avoid redis interference
  if [ $? -ne 0 ]; then
    echo "Generating db snapshot failed."
    exit 1
  fi
elif [ "$1" == "test" ]; then
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
