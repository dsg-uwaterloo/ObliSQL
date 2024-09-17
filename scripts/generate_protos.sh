PROTOBUF_PATH=.././api

mkdir -p $PROTOBUF_PATH/executor
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/executor --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/executor --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/executor.proto

mkdir -p $PROTOBUF_PATH/loadbalancer
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/loadbalancer --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/loadbalancer --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/loadbalancer.proto

mkdir -p $PROTOBUF_PATH/resolver
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/resolver --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/resolver --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/resolver.proto
