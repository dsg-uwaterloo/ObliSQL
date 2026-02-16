#!/bin/bash
# redis_deploy.sh
#
# Usage:
#   ./redis_deploy.sh "10.0.0.1 10.0.0.2 10.0.0.3" 6380 /hdd1/haseeb/redisdata
#
# Arguments:
#   1. Space-separated list of IP addresses
#   2. Base port number (will increment for each host)
#   3. Data directory (created on each host)

IPS=($1)
BASE_PORT=$2
DATA_DIR=$3

if [[ -z "$1" || -z "$2" || -z "$3" ]]; then
  echo "Usage: $0 \"<ip1 ip2 ...>\" <base_port> <data_dir>"
  exit 1
fi

for i in "${!IPS[@]}"; do
  HOST=${IPS[$i]}
  PORT=$((BASE_PORT + i))
  echo "Deploying Redis on $HOST:$PORT"

  ssh $HOST "mkdir -p $DATA_DIR/$PORT"

  ssh $HOST "nohup redis-server --port $PORT \
      --dir $DATA_DIR/$PORT \
      --daemonize yes \
      --save \"\" --appendonly no > /tmp/redis_$PORT.log 2>&1 &"

  echo "Redis started on $HOST:$PORT"
done
