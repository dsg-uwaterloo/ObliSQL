#!/bin/bash

# Run the tests
echo "Running the tests..."
go test -v ./tests/oram_batching_test.go
go test -v ./tests/oram_fake_read_test.go

# Check if the tests ran successfully
if [ $? -ne 0 ]; then
    echo "Test execution failed."
    exit 1
fi

echo "Tests executed successfully."