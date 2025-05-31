#!/bin/bash

# Define paths and variables for Query 1, Query 2, and Query 3
QUERY1_DIR=~/obliviator/pointQuery
QUERY1_EXECUTABLE=./host/parallel
QUERY1_ENCLAVE=./enclave/parallel_enc.signed
QUERY1_TEST_FILE=/home/azureuser/obliviator/data/big_data_benchmark/bdb_query1.txt
QUERY1_PARAM_VALUE=32

QUERY2_DIR=~/obliviator/rangeQuery
QUERY2_EXECUTABLE=./host/parallel
QUERY2_ENCLAVE=./enclave/parallel_enc.signed
QUERY2_TEST_FILE=/home/azureuser/obliviator/data/big_data_benchmark/bdb_query1.txt
QUERY2_PARAM_VALUE=32

# Join Query (Query 3) with 3 parts
QUERY3_PART1_DIR=~/obliviator/joinQuery/3_1
QUERY3_PART1_EXECUTABLE=./host/parallel
QUERY3_PART1_ENCLAVE=./enclave/parallel_enc.signed
QUERY3_PART1_INPUT=/home/azureuser/obliviator/data/big_data_benchmark/bdb_query3_partA_step1_input.txt
QUERY3_PARAM_VALUE=32

QUERY3_PART2_DIR=~/obliviator/joinQuery/3_2
QUERY3_PART2_EXECUTABLE=./host/parallel
QUERY3_PART2_ENCLAVE=./enclave/parallel_enc.signed
# Input for part 2 is the output from part 1
QUERY3_PART2_INPUT=/home/azureuser/obliviator/data/big_data_benchmark/bdb_query3_step2_sample_input.txt


QUERY3_PART3_DIR=~/obliviator/joinQuery/3_3
QUERY3_PART3_EXECUTABLE=./host/parallel
QUERY3_PART3_INPUT=/home/azureuser/obliviator/data/big_data_benchmark/bdb_query3_step3_sample_input.txt
QUERY3_PART3_ENCLAVE=./enclave/parallel_enc.signed
# Input for part 3 is the output from part 2

WAN_LATENCY=0.005  # 5ms in seconds
RUNS_PER_QUERY=3
TOTAL_ITERATIONS=9

echo "=== Client-Server Simulation with Performance Metrics ==="
echo "Will run $RUNS_PER_QUERY iterations of each query type for a total of $TOTAL_ITERATIONS iterations"

# Initialize arrays for statistics
execution_times=()
computation_times=()
measured_times=()
query_types=()
total_time=0
total_comp_time=0
total_measured_time=0
total_throughput=0

# Create array of queries to run: 3 of each type
queries_to_run=(1 1 1 2 2 2 3 3 3)

# Track if we've built each query type
query1_built=false
query2_built=false
query3_built=false

# Run the client-server simulation
for (( i=1; i<=$TOTAL_ITERATIONS; i++ )); do
    # Get the query type for this iteration
    QUERY_TYPE=${queries_to_run[$((i-1))]}
    query_types+=($QUERY_TYPE)
    
    if [ $QUERY_TYPE -eq 1 ]; then
        # QUERY 1 - Point Query
        OBLIVIATOR_DIR=$QUERY1_DIR
        EXECUTABLE=$QUERY1_EXECUTABLE
        ENCLAVE=$QUERY1_ENCLAVE
        TEST_FILE=$QUERY1_TEST_FILE
        PARAM_VALUE=$QUERY1_PARAM_VALUE
        echo "Iteration $i/$TOTAL_ITERATIONS - Running Query 1 (Point Query)"
        
        # Navigate to the directory and build if needed
        cd $OBLIVIATOR_DIR
        
        # Only build once per query type
        if [ "$query1_built" = false ]; then
            echo "Building the application..."
            make clean
            make
            
            if [ $? -ne 0 ]; then
                echo "Build failed. Exiting."
                exit 1
            fi
            echo "Build successful."
            query1_built=true
        fi
        
        # Client side: Send request (simulate with a delay)
        echo "Simulating network request..."
        sleep $WAN_LATENCY
        
        # Server side: Process the request and measure execution time
        echo "Processing request..."
        start_time=$(date +%s.%N)
        result=$($EXECUTABLE $ENCLAVE $PARAM_VALUE $TEST_FILE 2>&1)
        end_time=$(date +%s.%N)
        
        echo "$result" | grep -E "Point Query Search Value:"
        
        # Calculate the actual measured execution time
        measured_time=$(echo "$end_time - $start_time" | bc)
        
        # Extract execution time from the program's output (reported by the program)
        computation_time=$(echo "$result" | grep -o '[0-9]*\.[0-9]*')
        computation_times+=($computation_time)
        total_comp_time=$(echo "$total_comp_time + $computation_time" | bc)
        
    elif [ $QUERY_TYPE -eq 2 ]; then
        # QUERY 2 - Range Query
        OBLIVIATOR_DIR=$QUERY2_DIR
        EXECUTABLE=$QUERY2_EXECUTABLE
        ENCLAVE=$QUERY2_ENCLAVE
        TEST_FILE=$QUERY2_TEST_FILE
        PARAM_VALUE=$QUERY2_PARAM_VALUE
        echo "Iteration $i/$TOTAL_ITERATIONS - Running Query 2 (Range Query)"
        
        # Navigate to the directory and build if needed
        cd $OBLIVIATOR_DIR
        
        # Only build once per query type
        if [ "$query2_built" = false ]; then
            echo "Building the application..."
            make clean
            make
            
            if [ $? -ne 0 ]; then
                echo "Build failed. Exiting."
                exit 1
            fi
            echo "Build successful."
            query2_built=true
        fi
        
        # Client side: Send request (simulate with a delay)
        echo "Simulating network request..."
        sleep $WAN_LATENCY
        
        # Server side: Process the request and measure execution time
        echo "Processing request..."
        start_time=$(date +%s.%N)
        result=$($EXECUTABLE $ENCLAVE $PARAM_VALUE $TEST_FILE 2>&1)
        end_time=$(date +%s.%N)
        
        echo "$result" | grep -E "Range Query Search Values:"
        
        # Calculate the actual measured execution time
        measured_time=$(echo "$end_time - $start_time" | bc)
        
        # Extract execution time from the program's output (reported by the program)
        computation_time=$(echo "$result" | grep -o '[0-9]*\.[0-9]*')
        computation_times+=($computation_time)
        total_comp_time=$(echo "$total_comp_time + $computation_time" | bc)
        
    else
        # QUERY 3 - Join Query (3-part process)
        echo "Iteration $i/$TOTAL_ITERATIONS - Running Query 3 (Join Query)"
        computation_time=0
        
        # PART 1
        cd $QUERY3_PART1_DIR
        # Build if needed
        if [ "$query3_built" = false ]; then
            echo "Building Join Query Part 1..."
            make clean
            make
            
            if [ $? -ne 0 ]; then
                echo "Build failed. Exiting."
                exit 1
            fi
            echo "Build successful."
        fi
        
        # Client side: Send request (simulate with a delay)
        echo "Simulating network request for Join Query Part 1..."
        sleep $WAN_LATENCY
        
        # Execute Part 1
        echo "Processing Join Query Part 1..."
        start_time=$(date +%s.%N)
        result1=$($QUERY3_PART1_EXECUTABLE $QUERY3_PART1_ENCLAVE $QUERY3_PARAM_VALUE $QUERY3_PART1_INPUT 2>&1)
        end_time=$(date +%s.%N)
        
        # Extract execution time from part 1 output
        part1_time=$(echo "$result1" | grep -o '[0-9]*\.[0-9]*')
        echo "Join Query Part 1 time: ${part1_time}s"
        computation_time=$(echo "$computation_time + $part1_time" | bc)
        
        # PART 2
        cd $QUERY3_PART2_DIR
        # Build if needed
        if [ "$query3_built" = false ]; then
            echo "Building Join Query Part 2..."
            make clean
            make
            
            if [ $? -ne 0 ]; then
                echo "Build failed. Exiting."
                exit 1
            fi
            echo "Build successful."
        fi
        
        # Execute Part 2
        echo "Processing Join Query Part 2..."
        # Use the output from part 1 as input
        start_time=$(date +%s.%N)
        result2=$($QUERY3_PART2_EXECUTABLE $QUERY3_PART2_ENCLAVE $QUERY3_PARAM_VALUE $QUERY3_PART2_INPUT 2>&1)
        end_time=$(date +%s.%N)
        
        # Extract execution time from part 2 output
        part2_time=$(echo "$result2" | grep -o '[0-9]*\.[0-9]*')
        echo "Join Query Part 2 time: ${part2_time}s"
        computation_time=$(echo "$computation_time + $part2_time" | bc)
        
        # PART 3
        cd $QUERY3_PART3_DIR
        # Build if needed
        if [ "$query3_built" = false ]; then
            echo "Building Join Query Part 3..."
            make clean
            make
            
            if [ $? -ne 0 ]; then
                echo "Build failed. Exiting."
                exit 1
            fi
            echo "Build successful."
            query3_built=true
        fi
        
        # Execute Part 3
        echo "Processing Join Query Part 3..."
        # Use the output from part 2 as input
        start_time=$(date +%s.%N)
        result3=$($QUERY3_PART3_EXECUTABLE $QUERY3_PART3_ENCLAVE $QUERY3_PARAM_VALUE $QUERY3_PART3_INPUT 2>&1)
        end_time=$(date +%s.%N)
        
        # Extract execution time from part 3 output
        part3_time=$(echo "$result3" | grep -o '[0-9]*\.[0-9]*')
        echo "Join Query Part 3 time: ${part3_time}s"
        computation_time=$(echo "$computation_time + $part3_time" | bc)
        
        echo "Total Join Query time (sum of all 3 parts): ${computation_time}s"
        
        # Use the total execution time across all three parts
        measured_time=$(echo "$end_time - $start_time" | bc)
        computation_times+=($computation_time)
        total_comp_time=$(echo "$total_comp_time + $computation_time" | bc)
    fi
    
    echo "Measured execution time: ${measured_time}s"
    echo "Program reported time: ${computation_time}s"
    
    # Server response back to client (simulate with another delay)
    echo "Simulating network response..."
    sleep $WAN_LATENCY
    
    # Calculate total round-trip time including network latency
    round_trip_time=$(echo "$computation_time + 2*$WAN_LATENCY" | bc)
    
    # Store the time for this iteration
    execution_times+=($round_trip_time)
    measured_times+=($measured_time)
    
    # Update totals
    total_time=$(echo "$total_time + $round_trip_time" | bc)
    total_measured_time=$(echo "$total_measured_time + $measured_time" | bc)
    
    # Calculate throughput as operations per second (1/time)
    throughput=$(echo "scale=6; 1 / $round_trip_time" | bc)
    total_throughput=$(echo "$total_throughput + $throughput" | bc)
    
    echo "Completed iteration $i - Program reported time: ${computation_time}s, Measured time: ${measured_time}s, Round-trip time: ${round_trip_time}s"
    echo "-----------------------------------"
done

# Calculate average latency, computation time, and throughput
avg_latency=$(echo "scale=6; $total_time / $TOTAL_ITERATIONS" | bc)
avg_comp_time=$(echo "scale=6; $total_comp_time / $TOTAL_ITERATIONS" | bc)
avg_measured_time=$(echo "scale=6; $total_measured_time / $TOTAL_ITERATIONS" | bc)
avg_throughput=$(echo "scale=6; $total_throughput / $TOTAL_ITERATIONS" | bc)

# Sort the execution times for percentile calculations
IFS=

# Count query distribution
query1_count=$(echo "${query_types[@]}" | tr ' ' '\n' | grep -c "1")
query2_count=$(echo "${query_types[@]}" | tr ' ' '\n' | grep -c "2")
query3_count=$(echo "${query_types[@]}" | tr ' ' '\n' | grep -c "3")

# Report results
echo ""
echo "=== Performance Results ==="
echo "Total iterations: $TOTAL_ITERATIONS"
echo "Query distribution: Query 1 (Point): $query1_count, Query 2 (Range): $query2_count, Query 3 (Join): $query3_count"
echo "WAN latency simulation: ${WAN_LATENCY}s (each way)"
echo ""
echo "=== Round-trip Time (including network latency) ==="
echo "Average latency: ${avg_latency}s"
echo "Average throughput: ${avg_throughput} ops/sec"
echo ""
echo "=== Computation Time (excluding network latency) ==="
echo "=== Program Reported Time ==="
echo "Average computation time (reported): ${avg_comp_time}s"
echo ""
echo "=== Measured Execution Time ==="
echo "Average measured time: ${avg_measured_time}s"

# Clean up
echo ""
echo "Cleaning up..."
cd $QUERY1_DIR && make clean > /dev/null 2>&1
cd $QUERY2_DIR && make clean > /dev/null 2>&1
cd $QUERY3_PART1_DIR && make clean > /dev/null 2>&1
cd $QUERY3_PART2_DIR && make clean > /dev/null 2>&1
cd $QUERY3_PART3_DIR && make clean > /dev/null 2>&1