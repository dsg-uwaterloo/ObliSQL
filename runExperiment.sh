#!/bin/bash

REPOSITORY_PATH="/hdd1/haseeb/RelationalWaffle"

BATCH_HANDLER_HOST="tem100"
OBLIVIOUS_PROXY_HOST="tem101"
RESOLVER_HOST="tem111"
CLIENT_HOST="tem112"

REDIS_HOST="tem102"
REDIS_PORT="6379"

PROXY_TYPE="Waffle" # Waffle or ORAM

# Define variable values (no spaces around '=')
B=1200
C=2
N=1
F=100
D=100000
R=800
NUM=1
Z=500
INFLIGHT=2000
DURATION=20

# Get current time
current_time=$(date +"%Y%m%d_%H%M%S")

# Remove any old log or pid files if they exist
echo "Cleaning up old log and PID files..."
ssh -n "$OBLIVIOUS_PROXY_HOST" "rm -f $REPOSITORY_PATH/proxy_*.log $REPOSITORY_PATH/proxy_*.pid"
ssh -n "$BATCH_HANDLER_HOST" "rm -f $REPOSITORY_PATH/loadbalancer_*.log $REPOSITORY_PATH/loadbalancer_*.pid"
ssh -n "$RESOLVER_HOST" "rm -f $REPOSITORY_PATH/resolver_*.log $REPOSITORY_PATH/resolver_*.pid"
ssh -n "$CLIENT_HOST" "rm -f $REPOSITORY_PATH/benchmark_*.log $REPOSITORY_PATH/benchmark_*.pid"

echo "Killing all if they exist..."
ssh -n "$OBLIVIOUS_PROXY_HOST" "pkill -f proxy_server"
ssh -n "$BATCH_HANDLER_HOST" "pkill -f loadbalancer"
ssh -n "$RESOLVER_HOST" "pkill -f resolver"
ssh -n "$CLIENT_HOST" "pkill -f benchmark_client"



# Run the Oblivious Proxy on the remote host in the background
ssh -n "$OBLIVIOUS_PROXY_HOST" "bash -c '
(
    cd \"$REPOSITORY_PATH/waffle\" && \
    echo \"Building Proxy\" && \
    sh build.sh && \
    echo \"Built Waffle Proxy at: \$(pwd)\" && \
    ./bin/proxy_server -h $REDIS_HOST -p $REDIS_PORT
) > \"$REPOSITORY_PATH/proxy_${current_time}.log\" 2>&1 &
echo \$! > \"$REPOSITORY_PATH/proxy_${current_time}.pid\"
echo \"Proxy process started. Log: $REPOSITORY_PATH/proxy_${current_time}.log, PID: $REPOSITORY_PATH/proxy_${current_time}.pid\"
'" &


echo "Waiting for oblivious proxy on $OBLIVIOUS_PROXY_HOST to start up..."
sleep 10

# Run the loadbalancer on the remote host in the background
ssh -n "$BATCH_HANDLER_HOST" "bash -c '
(
    cd \"$REPOSITORY_PATH/pkg/loadbalancer\" && \
    go build && \
    echo \"Built executable at: \$(pwd)/loadbalancer\" && \
    ./loadbalancer -num $NUM -T $PROXY_TYPE -R $R -Z $Z -B $B -F $F -C $C -N $N -D $D -hosts $OBLIVIOUS_PROXY_HOST -ports 9090
) > \"$REPOSITORY_PATH/loadbalancer_${current_time}.log\" 2>&1 &
echo \$! > \"$REPOSITORY_PATH/loadbalancer_${current_time}.pid\"
echo \"Loadbalancer process started. Log: $REPOSITORY_PATH/loadbalancer_${current_time}.log, PID: $REPOSITORY_PATH/loadbalancer_${current_time}.pid\"
'" &

echo "Waiting for batch manager on $BATCH_HANDLER_HOST to start up..."
sleep 10

# Run the resolver on the remote host in the background
ssh -n "$RESOLVER_HOST" "bash -c '
(
    cd \"$REPOSITORY_PATH/pkg/resolver\" && \
    go build && \
    echo \"Built executable at: \$(pwd)/resolver\" && \
    ./resolver -h $BATCH_HANDLER_HOST -p 9500
) > \"$REPOSITORY_PATH/resolver_${current_time}.log\" 2>&1 &
echo \$! > \"$REPOSITORY_PATH/resolver_${current_time}.pid\"
echo \"Resolver process started. Log: $REPOSITORY_PATH/resolver_${current_time}.log, PID: $REPOSITORY_PATH/resolver_${current_time}.pid\"
'" &

echo "Waiting for resolver on $RESOLVER_HOST to start up..."
sleep 10

echo "Script completed. Both processes should now be running in the background on their respective hosts."
# echo "Logs and PIDs are saved in the root of $REPOSITORY_PATH on each host."
# echo "To check the logs, use:"
# echo "ssh $OBLIVIOUS_PROXY_HOST \"tail -f $REPOSITORY_PATH/proxy_${current_time}.log\""
# echo "ssh $BATCH_HANDLER_HOST \"tail -f $REPOSITORY_PATH/loadbalancer_${current_time}.log\""



echo "---------------------"

echo -e "\n"
echo "Running Benchmark"

# Run the Benchmark Client on the remote host in the background
ssh -T "$CLIENT_HOST" "bash -c '
(
    cd \"$REPOSITORY_PATH/pkg/benchmark_client\" && \
    go build && \
    echo \"Built executable at: \$(pwd)/benchmark_client\" && \
    ./benchmark_client -h $RESOLVER_HOST -p 9900 -s $INFLIGHT
)'" | tee "./benchmark_results_${current_time}.txt"

echo "Benchmark results saved to ./benchmark_results_${current_time}.txt"

echo "Killing all processes..."
ssh -n "$OBLIVIOUS_PROXY_HOST" "pkill -F \"$REPOSITORY_PATH/proxy_${current_time}.pid\" && rm \"$REPOSITORY_PATH/proxy_${current_time}.pid\""
ssh -n "$BATCH_HANDLER_HOST" "pkill -F \"$REPOSITORY_PATH/loadbalancer_${current_time}.pid\" && rm \"$REPOSITORY_PATH/loadbalancer_${current_time}.pid\""
ssh -n "$RESOLVER_HOST" "pkill -F \"$REPOSITORY_PATH/resolver_${current_time}.pid\" && rm \"$REPOSITORY_PATH/resolver_${current_time}.pid\""
ssh -n "$CLIENT_HOST" "pkill -F \"$REPOSITORY_PATH/benchmark_${current_time}.pid\" && rm \"$REPOSITORY_PATH/benchmark_${current_time}.pid\""

echo "All processes have been killed and PIDs have been cleaned up."