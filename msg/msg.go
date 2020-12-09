package msg

import "github.com/golang/protobuf/proto"

const CLIENT_MSG_HEARTBEAT = 1
const CLIENT_MSG_SYNC_CI_REQUEST = 2
const SERVER_MSG_CLIENT_CI_INFO = 3
const CLIENT_MSG_SYNC_CI_RESULT = 4
const SERVER_MSG_CLIENT_SYNC_CI_INFO = 5
const SERVER_MSG_HEARTBEAT_RESPONSE = 6

// Msg ...
// 消息
type Msg struct {
	Type     uint64
	RawDatas []byte
	Msg      proto.Message
}
