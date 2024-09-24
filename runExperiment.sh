#!/bin/bash

REPOSITORY_PATH="/hdd1/haseeb/RelationalWaffle"

OBLIVIOUS_PROXY_HOST="tem101"
BATCH_HANDLER_HOST="tem100"
RESOLVER_HOST="tem101"
CLIENT_HOST="tem102"

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

# Get current time
current_time=$(date +"%Y%m%d_%H%M%S")

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
    ./loadbalancer -num $NUM -T $PROXY_TYPE -R $R -Z $Z -B $B -F $F -C $C -N $N -D $D
) > \"$REPOSITORY_PATH/loadbalancer_${current_time}.log\" 2>&1 &
echo \$! > \"$REPOSITORY_PATH/loadbalancer_${current_time}.pid\"
echo \"Loadbalancer process started. Log: $REPOSITORY_PATH/loadbalancer_${current_time}.log, PID: $REPOSITORY_PATH/loadbalancer_${current_time}.pid\"
'" &

echo "Waiting for batch manager on $BATCH_HANDLER_HOST to start up..."
sleep 10


echo "Script completed. Both processes should now be running in the background on their respective hosts."
echo "Logs and PIDs are saved in the root of $REPOSITORY_PATH on each host."
echo "To check the logs, use:"
echo "ssh $OBLIVIOUS_PROXY_HOST \"tail -f $REPOSITORY_PATH/proxy_${current_time}.log\""
echo "ssh $BATCH_HANDLER_HOST \"tail -f $REPOSITORY_PATH/loadbalancer_${current_time}.log\""