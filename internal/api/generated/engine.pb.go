// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v5.29.3
// source: engine.proto

package generated

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CreateRunRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	YamlPayload   []byte                 `protobuf:"bytes,1,opt,name=yaml_payload,json=yamlPayload,proto3" json:"yaml_payload,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateRunRequest) Reset() {
	*x = CreateRunRequest{}
	mi := &file_engine_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateRunRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateRunRequest) ProtoMessage() {}

func (x *CreateRunRequest) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateRunRequest.ProtoReflect.Descriptor instead.
func (*CreateRunRequest) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{0}
}

func (x *CreateRunRequest) GetYamlPayload() []byte {
	if x != nil {
		return x.YamlPayload
	}
	return nil
}

type CreateRunResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RunId         string                 `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateRunResponse) Reset() {
	*x = CreateRunResponse{}
	mi := &file_engine_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateRunResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateRunResponse) ProtoMessage() {}

func (x *CreateRunResponse) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateRunResponse.ProtoReflect.Descriptor instead.
func (*CreateRunResponse) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{1}
}

func (x *CreateRunResponse) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

type LogStreamRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RunId         string                 `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LogStreamRequest) Reset() {
	*x = LogStreamRequest{}
	mi := &file_engine_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LogStreamRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogStreamRequest) ProtoMessage() {}

func (x *LogStreamRequest) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogStreamRequest.ProtoReflect.Descriptor instead.
func (*LogStreamRequest) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{2}
}

func (x *LogStreamRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

type LogLine struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Ts            string                 `protobuf:"bytes,1,opt,name=ts,proto3" json:"ts,omitempty"`
	Msg           string                 `protobuf:"bytes,2,opt,name=msg,proto3" json:"msg,omitempty"`
	Color         string                 `protobuf:"bytes,3,opt,name=color,proto3" json:"color,omitempty"` // "green" | "red" | "purple" | "" (default)
	Bold          bool                   `protobuf:"varint,4,opt,name=bold,proto3" json:"bold,omitempty"`
	TestName      string                 `protobuf:"bytes,5,opt,name=test_name,json=testName,proto3" json:"test_name,omitempty"` // Name of the test this log belongs to
	StepName      string                 `protobuf:"bytes,6,opt,name=step_name,json=stepName,proto3" json:"step_name,omitempty"` // Name of the step this log belongs to (if applicable)
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *LogLine) Reset() {
	*x = LogLine{}
	mi := &file_engine_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *LogLine) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogLine) ProtoMessage() {}

func (x *LogLine) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogLine.ProtoReflect.Descriptor instead.
func (*LogLine) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{3}
}

func (x *LogLine) GetTs() string {
	if x != nil {
		return x.Ts
	}
	return ""
}

func (x *LogLine) GetMsg() string {
	if x != nil {
		return x.Msg
	}
	return ""
}

func (x *LogLine) GetColor() string {
	if x != nil {
		return x.Color
	}
	return ""
}

func (x *LogLine) GetBold() bool {
	if x != nil {
		return x.Bold
	}
	return false
}

func (x *LogLine) GetTestName() string {
	if x != nil {
		return x.TestName
	}
	return ""
}

func (x *LogLine) GetStepName() string {
	if x != nil {
		return x.StepName
	}
	return ""
}

type ListRunsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRunsRequest) Reset() {
	*x = ListRunsRequest{}
	mi := &file_engine_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRunsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRunsRequest) ProtoMessage() {}

func (x *ListRunsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRunsRequest.ProtoReflect.Descriptor instead.
func (*ListRunsRequest) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{4}
}

type ListRunsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Runs          []*RunSummary          `protobuf:"bytes,1,rep,name=runs,proto3" json:"runs,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRunsResponse) Reset() {
	*x = ListRunsResponse{}
	mi := &file_engine_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRunsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRunsResponse) ProtoMessage() {}

func (x *ListRunsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRunsResponse.ProtoReflect.Descriptor instead.
func (*ListRunsResponse) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{5}
}

func (x *ListRunsResponse) GetRuns() []*RunSummary {
	if x != nil {
		return x.Runs
	}
	return nil
}

type RunSummary struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RunId         string                 `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	Status        string                 `protobuf:"bytes,2,opt,name=status,proto3" json:"status,omitempty"` // PENDING | RUNNING | PASSED | FAILED
	StartedAt     string                 `protobuf:"bytes,3,opt,name=started_at,json=startedAt,proto3" json:"started_at,omitempty"`
	EndedAt       string                 `protobuf:"bytes,4,opt,name=ended_at,json=endedAt,proto3" json:"ended_at,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RunSummary) Reset() {
	*x = RunSummary{}
	mi := &file_engine_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RunSummary) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunSummary) ProtoMessage() {}

func (x *RunSummary) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunSummary.ProtoReflect.Descriptor instead.
func (*RunSummary) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{6}
}

func (x *RunSummary) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *RunSummary) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *RunSummary) GetStartedAt() string {
	if x != nil {
		return x.StartedAt
	}
	return ""
}

func (x *RunSummary) GetEndedAt() string {
	if x != nil {
		return x.EndedAt
	}
	return ""
}

type AddLogRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RunId         string                 `protobuf:"bytes,1,opt,name=run_id,json=runId,proto3" json:"run_id,omitempty"`
	WorkflowId    string                 `protobuf:"bytes,2,opt,name=workflow_id,json=workflowId,proto3" json:"workflow_id,omitempty"`
	Message       string                 `protobuf:"bytes,3,opt,name=message,proto3" json:"message,omitempty"`
	Color         string                 `protobuf:"bytes,4,opt,name=color,proto3" json:"color,omitempty"`
	Bold          bool                   `protobuf:"varint,5,opt,name=bold,proto3" json:"bold,omitempty"`
	TestName      string                 `protobuf:"bytes,6,opt,name=test_name,json=testName,proto3" json:"test_name,omitempty"` // Name of the test this log belongs to
	StepName      string                 `protobuf:"bytes,7,opt,name=step_name,json=stepName,proto3" json:"step_name,omitempty"` // Name of the step this log belongs to (if applicable)
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AddLogRequest) Reset() {
	*x = AddLogRequest{}
	mi := &file_engine_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AddLogRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddLogRequest) ProtoMessage() {}

func (x *AddLogRequest) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddLogRequest.ProtoReflect.Descriptor instead.
func (*AddLogRequest) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{7}
}

func (x *AddLogRequest) GetRunId() string {
	if x != nil {
		return x.RunId
	}
	return ""
}

func (x *AddLogRequest) GetWorkflowId() string {
	if x != nil {
		return x.WorkflowId
	}
	return ""
}

func (x *AddLogRequest) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *AddLogRequest) GetColor() string {
	if x != nil {
		return x.Color
	}
	return ""
}

func (x *AddLogRequest) GetBold() bool {
	if x != nil {
		return x.Bold
	}
	return false
}

func (x *AddLogRequest) GetTestName() string {
	if x != nil {
		return x.TestName
	}
	return ""
}

func (x *AddLogRequest) GetStepName() string {
	if x != nil {
		return x.StepName
	}
	return ""
}

type AddLogResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AddLogResponse) Reset() {
	*x = AddLogResponse{}
	mi := &file_engine_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AddLogResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AddLogResponse) ProtoMessage() {}

func (x *AddLogResponse) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AddLogResponse.ProtoReflect.Descriptor instead.
func (*AddLogResponse) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{8}
}

type HealthRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthRequest) Reset() {
	*x = HealthRequest{}
	mi := &file_engine_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthRequest) ProtoMessage() {}

func (x *HealthRequest) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthRequest.ProtoReflect.Descriptor instead.
func (*HealthRequest) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{9}
}

type HealthResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        string                 `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"` // "ok" | "error"
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthResponse) Reset() {
	*x = HealthResponse{}
	mi := &file_engine_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthResponse) ProtoMessage() {}

func (x *HealthResponse) ProtoReflect() protoreflect.Message {
	mi := &file_engine_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthResponse.ProtoReflect.Descriptor instead.
func (*HealthResponse) Descriptor() ([]byte, []int) {
	return file_engine_proto_rawDescGZIP(), []int{10}
}

func (x *HealthResponse) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

var File_engine_proto protoreflect.FileDescriptor

const file_engine_proto_rawDesc = "" +
	"\n" +
	"\fengine.proto\x12\rrocketship.v1\"5\n" +
	"\x10CreateRunRequest\x12!\n" +
	"\fyaml_payload\x18\x01 \x01(\fR\vyamlPayload\"*\n" +
	"\x11CreateRunResponse\x12\x15\n" +
	"\x06run_id\x18\x01 \x01(\tR\x05runId\")\n" +
	"\x10LogStreamRequest\x12\x15\n" +
	"\x06run_id\x18\x01 \x01(\tR\x05runId\"\x8f\x01\n" +
	"\aLogLine\x12\x0e\n" +
	"\x02ts\x18\x01 \x01(\tR\x02ts\x12\x10\n" +
	"\x03msg\x18\x02 \x01(\tR\x03msg\x12\x14\n" +
	"\x05color\x18\x03 \x01(\tR\x05color\x12\x12\n" +
	"\x04bold\x18\x04 \x01(\bR\x04bold\x12\x1b\n" +
	"\ttest_name\x18\x05 \x01(\tR\btestName\x12\x1b\n" +
	"\tstep_name\x18\x06 \x01(\tR\bstepName\"\x11\n" +
	"\x0fListRunsRequest\"A\n" +
	"\x10ListRunsResponse\x12-\n" +
	"\x04runs\x18\x01 \x03(\v2\x19.rocketship.v1.RunSummaryR\x04runs\"u\n" +
	"\n" +
	"RunSummary\x12\x15\n" +
	"\x06run_id\x18\x01 \x01(\tR\x05runId\x12\x16\n" +
	"\x06status\x18\x02 \x01(\tR\x06status\x12\x1d\n" +
	"\n" +
	"started_at\x18\x03 \x01(\tR\tstartedAt\x12\x19\n" +
	"\bended_at\x18\x04 \x01(\tR\aendedAt\"\xc5\x01\n" +
	"\rAddLogRequest\x12\x15\n" +
	"\x06run_id\x18\x01 \x01(\tR\x05runId\x12\x1f\n" +
	"\vworkflow_id\x18\x02 \x01(\tR\n" +
	"workflowId\x12\x18\n" +
	"\amessage\x18\x03 \x01(\tR\amessage\x12\x14\n" +
	"\x05color\x18\x04 \x01(\tR\x05color\x12\x12\n" +
	"\x04bold\x18\x05 \x01(\bR\x04bold\x12\x1b\n" +
	"\ttest_name\x18\x06 \x01(\tR\btestName\x12\x1b\n" +
	"\tstep_name\x18\a \x01(\tR\bstepName\"\x10\n" +
	"\x0eAddLogResponse\"\x0f\n" +
	"\rHealthRequest\"(\n" +
	"\x0eHealthResponse\x12\x16\n" +
	"\x06status\x18\x01 \x01(\tR\x06status2\xfc\x02\n" +
	"\x06Engine\x12N\n" +
	"\tCreateRun\x12\x1f.rocketship.v1.CreateRunRequest\x1a .rocketship.v1.CreateRunResponse\x12G\n" +
	"\n" +
	"StreamLogs\x12\x1f.rocketship.v1.LogStreamRequest\x1a\x16.rocketship.v1.LogLine0\x01\x12E\n" +
	"\x06AddLog\x12\x1c.rocketship.v1.AddLogRequest\x1a\x1d.rocketship.v1.AddLogResponse\x12K\n" +
	"\bListRuns\x12\x1e.rocketship.v1.ListRunsRequest\x1a\x1f.rocketship.v1.ListRunsResponse\x12E\n" +
	"\x06Health\x12\x1c.rocketship.v1.HealthRequest\x1a\x1d.rocketship.v1.HealthResponseB9Z7github.com/rocketship/rocketship/internal/api/generatedb\x06proto3"

var (
	file_engine_proto_rawDescOnce sync.Once
	file_engine_proto_rawDescData []byte
)

func file_engine_proto_rawDescGZIP() []byte {
	file_engine_proto_rawDescOnce.Do(func() {
		file_engine_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_engine_proto_rawDesc), len(file_engine_proto_rawDesc)))
	})
	return file_engine_proto_rawDescData
}

var file_engine_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_engine_proto_goTypes = []any{
	(*CreateRunRequest)(nil),  // 0: rocketship.v1.CreateRunRequest
	(*CreateRunResponse)(nil), // 1: rocketship.v1.CreateRunResponse
	(*LogStreamRequest)(nil),  // 2: rocketship.v1.LogStreamRequest
	(*LogLine)(nil),           // 3: rocketship.v1.LogLine
	(*ListRunsRequest)(nil),   // 4: rocketship.v1.ListRunsRequest
	(*ListRunsResponse)(nil),  // 5: rocketship.v1.ListRunsResponse
	(*RunSummary)(nil),        // 6: rocketship.v1.RunSummary
	(*AddLogRequest)(nil),     // 7: rocketship.v1.AddLogRequest
	(*AddLogResponse)(nil),    // 8: rocketship.v1.AddLogResponse
	(*HealthRequest)(nil),     // 9: rocketship.v1.HealthRequest
	(*HealthResponse)(nil),    // 10: rocketship.v1.HealthResponse
}
var file_engine_proto_depIdxs = []int32{
	6,  // 0: rocketship.v1.ListRunsResponse.runs:type_name -> rocketship.v1.RunSummary
	0,  // 1: rocketship.v1.Engine.CreateRun:input_type -> rocketship.v1.CreateRunRequest
	2,  // 2: rocketship.v1.Engine.StreamLogs:input_type -> rocketship.v1.LogStreamRequest
	7,  // 3: rocketship.v1.Engine.AddLog:input_type -> rocketship.v1.AddLogRequest
	4,  // 4: rocketship.v1.Engine.ListRuns:input_type -> rocketship.v1.ListRunsRequest
	9,  // 5: rocketship.v1.Engine.Health:input_type -> rocketship.v1.HealthRequest
	1,  // 6: rocketship.v1.Engine.CreateRun:output_type -> rocketship.v1.CreateRunResponse
	3,  // 7: rocketship.v1.Engine.StreamLogs:output_type -> rocketship.v1.LogLine
	8,  // 8: rocketship.v1.Engine.AddLog:output_type -> rocketship.v1.AddLogResponse
	5,  // 9: rocketship.v1.Engine.ListRuns:output_type -> rocketship.v1.ListRunsResponse
	10, // 10: rocketship.v1.Engine.Health:output_type -> rocketship.v1.HealthResponse
	6,  // [6:11] is the sub-list for method output_type
	1,  // [1:6] is the sub-list for method input_type
	1,  // [1:1] is the sub-list for extension type_name
	1,  // [1:1] is the sub-list for extension extendee
	0,  // [0:1] is the sub-list for field type_name
}

func init() { file_engine_proto_init() }
func file_engine_proto_init() {
	if File_engine_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_engine_proto_rawDesc), len(file_engine_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_engine_proto_goTypes,
		DependencyIndexes: file_engine_proto_depIdxs,
		MessageInfos:      file_engine_proto_msgTypes,
	}.Build()
	File_engine_proto = out.File
	file_engine_proto_goTypes = nil
	file_engine_proto_depIdxs = nil
}
