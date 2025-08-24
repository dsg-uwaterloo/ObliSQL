#!/bin/bash

# Run the tests
echo "Starting the redis server in the background..."
redis-server --daemonize yes

# Check if the tests ran successfully
if [ $? -ne 0 ]; then
    echo "Failed to start redis server."
    exit 1
fi
