// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v3.12.4
// source: loadbalancer.proto

package loadbalancer

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LoadBalanceRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RequestId    int64    `protobuf:"varint,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	ObjectNum    int64    `protobuf:"varint,2,opt,name=object_num,json=objectNum,proto3" json:"object_num,omitempty"`
	TotalObjects int64    `protobuf:"varint,3,opt,name=totalObjects,proto3" json:"totalObjects,omitempty"`
	Keys         []string `protobuf:"bytes,4,rep,name=keys,proto3" json:"keys,omitempty"`
	Values       []string `protobuf:"bytes,5,rep,name=values,proto3" json:"values,omitempty"`
}

func (x *LoadBalanceRequest) Reset() {
	*x = LoadBalanceRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LoadBalanceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LoadBalanceRequest) ProtoMessage() {}

func (x *LoadBalanceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LoadBalanceRequest.ProtoReflect.Descriptor instead.
func (*LoadBalanceRequest) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{0}
}

func (x *LoadBalanceRequest) GetRequestId() int64 {
	if x != nil {
		return x.RequestId
	}
	return 0
}

func (x *LoadBalanceRequest) GetObjectNum() int64 {
	if x != nil {
		return x.ObjectNum
	}
	return 0
}

func (x *LoadBalanceRequest) GetTotalObjects() int64 {
	if x != nil {
		return x.TotalObjects
	}
	return 0
}

func (x *LoadBalanceRequest) GetKeys() []string {
	if x != nil {
		return x.Keys
	}
	return nil
}

func (x *LoadBalanceRequest) GetValues() []string {
	if x != nil {
		return x.Values
	}
	return nil
}

type LoadBalanceResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RequestId    int64    `protobuf:"varint,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	ObjectNum    int64    `protobuf:"varint,2,opt,name=object_num,json=objectNum,proto3" json:"object_num,omitempty"`
	TotalObjects int64    `protobuf:"varint,3,opt,name=totalObjects,proto3" json:"totalObjects,omitempty"`
	Keys         []string `protobuf:"bytes,4,rep,name=keys,proto3" json:"keys,omitempty"`
	Values       []string `protobuf:"bytes,5,rep,name=values,proto3" json:"values,omitempty"`
}

func (x *LoadBalanceResponse) Reset() {
	*x = LoadBalanceResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_loadbalancer_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LoadBalanceResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LoadBalanceResponse) ProtoMessage() {}

func (x *LoadBalanceResponse) ProtoReflect() protoreflect.Message {
	mi := &file_loadbalancer_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LoadBalanceResponse.ProtoReflect.Descriptor instead.
func (*LoadBalanceResponse) Descriptor() ([]byte, []int) {
	return file_loadbalancer_proto_rawDescGZIP(), []int{1}
}

func (x *LoadBalanceResponse) GetRequestId() int64 {
	if x != nil {
		return x.RequestId
	}
	return 0
}

func (x *LoadBalanceResponse) GetObjectNum() int64 {
	if x != nil {
		return x.ObjectNum
	}
	return 0
}

func (x *LoadBalanceResponse) GetTotalObjects() int64 {
	if x != nil {
		return x.TotalObjects
	}
	return 0
}

func (x *LoadBalanceResponse) GetKeys() []string {
	if x != nil {
		return x.Keys
	}
	return nil
}

func (x *LoadBalanceResponse) GetValues() []string {
	if x != nil {
		return x.Values
	}
	return nil
}

var File_loadbalancer_proto protoreflect.FileDescriptor

var file_loadbalancer_proto_rawDesc = []byte{
	0x0a, 0x12, 0x6c, 0x6f, 0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xa2, 0x01, 0x0a, 0x12, 0x6c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c,
	0x61, 0x6e, 0x63, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x72,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x09, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x6f, 0x62,
	0x6a, 0x65, 0x63, 0x74, 0x5f, 0x6e, 0x75, 0x6d, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09,
	0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x4e, 0x75, 0x6d, 0x12, 0x22, 0x0a, 0x0c, 0x74, 0x6f, 0x74,
	0x61, 0x6c, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x0c, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x12, 0x12, 0x0a,
	0x04, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x6b, 0x65, 0x79,
	0x73, 0x12, 0x16, 0x0a, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x22, 0xa3, 0x01, 0x0a, 0x13, 0x6c, 0x6f,
	0x61, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x49, 0x64,
	0x12, 0x1d, 0x0a, 0x0a, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x5f, 0x6e, 0x75, 0x6d, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x4e, 0x75, 0x6d, 0x12,
	0x22, 0x0a, 0x0c, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x4f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0c, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x4f, 0x62, 0x6a, 0x65,
	0x63, 0x74, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28,
	0x09, 0x52, 0x04, 0x6b, 0x65, 0x79, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x73, 0x32,
	0x44, 0x0a, 0x0c, 0x4c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x12,
	0x34, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x4b, 0x65, 0x79, 0x73, 0x12, 0x13, 0x2e, 0x6c, 0x6f, 0x61,
	0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x14, 0x2e, 0x6c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x23, 0x5a, 0x21, 0x4c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c,
	0x61, 0x6e, 0x63, 0x65, 0x72, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x3b, 0x6c, 0x6f,
	0x61, 0x64, 0x62, 0x61, 0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_loadbalancer_proto_rawDescOnce sync.Once
	file_loadbalancer_proto_rawDescData = file_loadbalancer_proto_rawDesc
)

func file_loadbalancer_proto_rawDescGZIP() []byte {
	file_loadbalancer_proto_rawDescOnce.Do(func() {
		file_loadbalancer_proto_rawDescData = protoimpl.X.CompressGZIP(file_loadbalancer_proto_rawDescData)
	})
	return file_loadbalancer_proto_rawDescData
}

var file_loadbalancer_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_loadbalancer_proto_goTypes = []any{
	(*LoadBalanceRequest)(nil),  // 0: loadBalanceRequest
	(*LoadBalanceResponse)(nil), // 1: loadBalanceResponse
}
var file_loadbalancer_proto_depIdxs = []int32{
	0, // 0: LoadBalancer.addKeys:input_type -> loadBalanceRequest
	1, // 1: LoadBalancer.addKeys:output_type -> loadBalanceResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_loadbalancer_proto_init() }
func file_loadbalancer_proto_init() {
	if File_loadbalancer_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_loadbalancer_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*LoadBalanceRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_loadbalancer_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*LoadBalanceResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_loadbalancer_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_loadbalancer_proto_goTypes,
		DependencyIndexes: file_loadbalancer_proto_depIdxs,
		MessageInfos:      file_loadbalancer_proto_msgTypes,
	}.Build()
	File_loadbalancer_proto = out.File
	file_loadbalancer_proto_rawDesc = nil
	file_loadbalancer_proto_goTypes = nil
	file_loadbalancer_proto_depIdxs = nil
}