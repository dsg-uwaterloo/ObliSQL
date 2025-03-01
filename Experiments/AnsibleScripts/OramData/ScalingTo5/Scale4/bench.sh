#!/bin/bash

# ===============================================================
# Script: run_benchmark.sh
# Description: Wrapper script to run the Ansible playbook with
#              varying -s parameters for the benchmark command.
# Usage: ./run_benchmark.sh --s-values 4000 6000 8000 10000 --prefix <output_prefix>
# Example: ./run_benchmark.sh --s-values 4000 6000 8000 10000 --prefix result
# ===============================================================

# Default values
PREFIX=""  # Default prefix for output files
S_VALUES=()  # Default -s values

# Function to display usage
usage() {
    echo "Usage: $0 [--s-values <s1> <s2> ... <sn>] [--prefix <output_prefix>] [--help]"
    echo ""
    echo "Options:"
    echo "  --s-values    Space-separated list of -s parameter values (default: 4000 6000 8000 10000)"
    echo "  --prefix      Prefix for the benchmark output files (default: benchmark_output)"
    echo "  -h, --help    Display this help message"
    exit 1
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --s-values)
            shift
            # Read all following arguments until the next option (starts with --)
            while [[ $# -gt 0 && ! "$1" =~ ^-- ]]; do
                S_VALUES+=("$1")
                shift
            done
            ;;
        --prefix)
            PREFIX="$2"
            shift
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)    # unknown option
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Check if S_VALUES is not empty
if [ ${#S_VALUES[@]} -eq 0 ]; then
    echo "Error: No -s values provided."
    usage
fi

# Iterate over each -s value and run the playbook
for S_VALUE in "${S_VALUES[@]}"; do
    # Calculate B, R and F_D based on B_R
    C_SIZE=2
    SCALE_FACTOR=4
    N=$(echo "6095882 / $SCALE_FACTOR"| bc)
    B_R=30
    
    C=$(echo "scale=0; ($N * 2 / 100 + 0.5)/1" | bc)  # Use scale=0 for rounding up
    R=$B_R
    
    # Calculate B and F_D using bc for floating-point arithmetic
    B=$(echo "$R * 1/0.6" | bc)
    F_D=$(echo "$B * 5 / 100" | bc)

    fromAlpha=$(echo "scale=5; (($N - $C - $B + $F_D) / ($B - $R - $F_D))" | bc)
    D=$(echo "scale=5; $N * 0.13" | bc)
    D_scaled_up=$(echo "($D + 0.999999) / 1" | bc)


    
    
    echo "==============================================="
    echo "Running benchmark with -s $S_VALUE"
    echo "Alpha = $fromAlpha"
    echo "N = $N"
    echo "B_R = $B_R"
    echo "B = $B"
    echo "R = $R"
    echo "F_D = $F_D"
    echo "D = $D_scaled_up"
    echo "Cache Size = $C"
    echo "==============================================="
    # Construct the extra variables string for Ansible
    EXTRA_VARS="b='$B' r='$R' fd='$F_D' sf='$SCALE_FACTOR' d='$D_scaled_up'"


    # Construct the benchmark arguments
    BENCHMARK_ARGS="-s $S_VALUE -q scaling"

    # Construct the output filename
    OUTPUT_FILE="B_${B}_s_${S_VALUE}_${PREFIX}.txt"

    # Display the configuration being used
    echo "Benchmark Arguments: $BENCHMARK_ARGS"
    echo "Benchmark Output Filename: $OUTPUT_FILE"
    echo ""

    # # Execute the Ansible playbook with extra-vars
    ansible-playbook -i inventory.ini oramDeploy.yml \
        -e "benchmark_args='$BENCHMARK_ARGS'" \
        -e "$EXTRA_VARS" \
        -e "output_filename='$OUTPUT_FILE'"

    # Check if the playbook ran successfully
    if [ $? -eq 0 ]; then
        echo "Benchmark with -s $S_VALUE completed successfully."
        echo "Output saved to $OUTPUT_FILE."
    else
        echo "Benchmark with -s $S_VALUE failed. Check Ansible output for details."
        # Optionally, you can decide to exit or continue based on failure
        # exit 1
    fi

    echo ""  # Add an empty line for readability
done
echo "All benchmarks completed."