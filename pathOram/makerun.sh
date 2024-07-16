#!/bin/bash

# Set the Go source file for your main program
MAIN_FILE="main.go"

# Set the name for the executable
EXECUTABLE="main"

# Build the Go program
echo "Building the Go program..."
sudo go build -o $EXECUTABLE $MAIN_FILE

# Check if the build was successful
if [ $? -ne 0 ]; then
    echo "Build failed."
    exit 1
fi

echo "Build succeeded."

# Run the executable
echo "Running the program..."
./$EXECUTABLE

# Check if the program ran successfully
if [ $? -ne 0 ]; then
    echo "Program execution failed."
    exit 1
fi

echo "Program executed successfully."
