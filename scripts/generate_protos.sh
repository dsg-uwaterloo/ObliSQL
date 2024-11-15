PROTOBUF_PATH=.././api

# Plaintext Executor
mkdir -p $PROTOBUF_PATH/plaintextExecutor
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/plaintextExecutor --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/plaintextExecutor --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/plainTextExecutor.proto

mkdir -p $PROTOBUF_PATH/loadbalancer
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/loadbalancer --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/loadbalancer --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/loadbalancer.proto

mkdir -p $PROTOBUF_PATH/resolver
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/resolver --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/resolver --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/resolver.proto

mkdir -p $PROTOBUF_PATH/oramExecutor
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/oramExecutor --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/oramExecutor --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/oramExecutor.proto
