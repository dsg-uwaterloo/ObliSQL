PROTOBUF_PATH=.././api

mkdir -p $PROTOBUF_PATH/redis
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/redis --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/redis --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/redis.proto

mkdir -p $PROTOBUF_PATH/waffle
protoc --proto_path=$PROTOBUF_PATH --go_out=$PROTOBUF_PATH/waffle --go_opt=paths=source_relative --go-grpc_out=$PROTOBUF_PATH/waffle --go-grpc_opt=paths=source_relative $PROTOBUF_PATH/waffle.proto
