package msg

import "github.com/golang/protobuf/proto"

const CLIENT_MSG_HEARTBEAT = 1
const CLIENT_MSG_COLLECT = 2

// Msg ...
// 消息
type Msg struct {
	Type     uint64
	RawDatas []byte
	Msg      proto.Message
}
