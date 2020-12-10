package core

import (
	"context"
	"microagent/common"
	ae "microagent/common/error"
	cfg "microagent/common/configparse"
	log "microagent/common/formatlog"
	"microagent/msg"
	"net"
	"sync"
	"time"
	"bytes"
	"github.com/golang/protobuf/proto"
	"fmt"
)

const (
	Waiting = iota
	Running
	Erroring
)

type FutureErrors struct {
	ErrorSlice []error
}

type InternalMsg struct {
	Lock *sync.RWMutex
	Msg  map[string]interface{}
}

// agent结构体，插件方式，每个future 作为一个interface进行实现，future和主agent通过chan event进行
type Agent struct {
	futures           map[string]Future  //支持的future map
	cancel            context.CancelFunc // ctx cancel
	ctx               context.Context    // ctx
	state             int                // agent 状态字段
	conn              net.Conn           // agent的Conn
	sendLock		  *sync.Mutex		 // 发送锁
	readBuf           []byte             // 读取的缓存
	readMsgPayloadLth uint64             // 读取的当前消息的长度
	readTotalBytesLth uint64             // 读取的总的消息长度
}

// future interface，每个future必须要实现内部的几个方法
type Future interface {
	Init() error
	Start(agtCtx context.Context, chMsg chan *InternalMsg) error
	Stop() error
	Destory() error
}

// Agent初始化
func NewAgent() *Agent {
	log.Infoln("[Agent] 初始化Agent 对象....")
	agt := Agent{
		futures: map[string]Future{},
		state:   Waiting,
		conn:	nil,
		sendLock:	&sync.Mutex{},
		readBuf:	[]byte{},
		readMsgPayloadLth:	0,
		readTotalBytesLth:	0,
	}
	return &agt
}

// Agent初始化
func (agt *Agent) reset() {
	log.Infoln("[Agent] 重置Agent 对象....")
	// 关闭所有插件协程
	//agt.cancel()
	//agt.stopFutures()

	// 重置Agent 对象
	agt.futures = map[string]Future{}
	agt.state = Waiting
	agt.conn = nil
	agt.sendLock = &sync.Mutex{}
	agt.readBuf = []byte{}
	agt.readMsgPayloadLth = 0
	agt.readTotalBytesLth = 0
}

// 注册future
func (agt *Agent) RegisterFuture(name string, fucture Future) error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.futures[name] = fucture
	return fucture.Init()
}

// 真正启动Agent，启动失败则重启Agent
func (agt *Agent) Run() {
	var err error
	for {
		log.Infoln("[Agent] 准备启动Agent....")
		if err = agt.Start(); err != nil {
			log.Infoln("[Agent] 启动Agent失败，等待10S 后进行重新启动")
			time.Sleep(time.Duration(10) * time.Second)
			agt.reset()
			continue
		} else {
			select {}
		}
	}
}

// 启动agent, 同时启动所有插件
func (agt *Agent) Start() error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.state = Running

	// 启动tcp 服务，和server 端的主要通信方式
	serviceAddr := cfg.GlobalConf.GetStr("common", "svraddr")
	conn, err := net.Dial("tcp", serviceAddr)
	if err != nil {
		log.Errorf("[Agent]无法连接到服务器 %s，报错信息 %s", serviceAddr, err.Error())
		return ae.New("[Agent状态异常] 连接Agent到服务端异常")
	}
	defer conn.Close()
	agt.conn = conn

	// 启动各个插件
	agt.ctx, agt.cancel = context.WithCancel(context.Background())

	return agt.startFutures()
}

// 新建一个协程，用于处理其他协程发来的消息
func (agt *Agent) FutureMsgProcessGroutine(agtCtx context.Context, chMsg chan *InternalMsg) {
	for {
		select {
		case fuMsg := <-chMsg:
			switch fuMsg.Msg["futname"] {
			case "Heartbeat":
				log.Infoln(*fuMsg)
			case "Collect":
				log.Infoln(*fuMsg)
			default:
				log.Errorln("[Agent]不存在的插件名, 请检查内部消息")
			}
		case <-agtCtx.Done():
			return
		}
	}
}

// 启动Futures
func (agt *Agent) startFutures() error {
	var err error
	var errs FutureErrors
	var errInfo string
	var msgChan = make(chan *InternalMsg, 20)

	// 启动个gorountine 读取内部插件的消息
	go agt.FutureMsgProcessGroutine(agt.ctx, msgChan)

	for name, future := range agt.futures {
		go func(name string, future Future, ctx context.Context, chMsg chan *InternalMsg) {
			log.Infof("[Agent] future: %s 准备启动....", name)
			if err = future.Start(agt.ctx, msgChan); err != nil {
				errs.ErrorSlice = append(errs.ErrorSlice, ae.New(name+":"+ err.Error()))
			}
		}(name, future, agt.ctx, msgChan)
		//go future.Start(agt.ctx, msgChan)
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	} else {
		for _, er := range(errs.ErrorSlice) {
			erInfo := fmt.Sprintf("[Agent] 插件启动错误, %v,", er.Error())
			errInfo = errInfo + erInfo
		}
		agt.Stop()
		return ae.New(errInfo)
	}
}

// 关闭agent
func (agt *Agent) Stop() error {
	if agt.state != Running {
		return ae.StateError()
	}
	agt.state = Waiting
	agt.cancel()
	return agt.stopFutures()
}

//停止future
func (agt *Agent) stopFutures() error {
	var err error
	var errs FutureErrors
	var errInfo string
	for name, future := range agt.futures {
		if err = future.Stop(); err != nil {
			errs.ErrorSlice = append(errs.ErrorSlice, ae.New(name+":"+ err.Error()))
		}
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	} else {
		for _, er := range(errs.ErrorSlice) {
			erInfo := fmt.Sprintf("[Agent] 插件关闭错误, %v,", er.Error())
			errInfo = errInfo + erInfo
		}
		return ae.New(errInfo)
	}
}

// 销毁agent
func (agt *Agent) Destory() error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	return agt.destoryFutures()
}

// 销毁futures
func (agt *Agent) destoryFutures() error {
	var err error
	var errs FutureErrors
	var errInfo string
	for name, future := range agt.futures {
		if err = future.Destory(); err != nil {
			errs.ErrorSlice = append(errs.ErrorSlice, ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	} else {
		for _, er := range(errs.ErrorSlice) {
			erInfo := fmt.Sprintf("[Agent] 插件销毁错误, %v,", er.Error())
			errInfo = errInfo + erInfo
		}
		return ae.New(errInfo)
	}
}

// 发送消息
func (agt *Agent) sendMsg(msg *msg.Msg) {
	// 生成MSG
	protobufMsg := []byte{}
	var err error
	if msg.Msg != nil {
		protobufMsg, err = proto.Marshal(msg.Msg)
		if err != nil {
			log.Errorf("protobuf消息生成失败: %s", err.Error())
			return
		}
	}
	// 计算长度等信息
	msgSize := len(protobufMsg)
	msgType := msg.Type

	// 生成结果报文
	packetBuf := &bytes.Buffer{}
	lengthBytes := common.GenLengthFromInt(msgSize)
	packetBuf.Write(lengthBytes[:])
	typeBytes := common.GenTypeFromInt(int(msgType))
	packetBuf.Write(typeBytes[:])
	packetBuf.Write(protobufMsg)
	packet := packetBuf.Bytes()

	log.Debugf("发送报文，报文长度 %d，类型 %d，消息体长度 %d", msgSize+12, msgType, msgSize)
	log.Debugf("报文内容: %v", packet)

	agt.sendLock.Lock()
	// 设置写入超时
	agt.conn.SetWriteDeadline(time.Now().Add(time.Duration(cfg.GlobalConf.GetInt("common", "sendwrtimeout")) * time.Second))
	_, err = agt.conn.Write(packet)
	if err != nil {
		log.Errorf("sendMsg失败: %s", err.Error())
		agt.Stop()
	}
	agt.sendLock.Unlock()
}

// 处理消息的方法
func (agt *Agent) getMsg() (*msg.Msg, error) {
	/*
		消息格式: 8字节protobuf报文长度 + 4字节类型 + protobuf报文
		读取逻辑:
		- 读取报文，将报文和client内部的buf相加，如果长度小于8则继续读取
		- 如果长度大于8则根据计算得到的长度读取所有数据
		- 读取后解析protobuf生成msg，并消耗相关的buf
	*/

	for {
		if agt.state == Erroring {
			return nil, ae.StateError()
		}

		// 设置读的超时时间
		agt.conn.SetReadDeadline(time.Now().Add(time.Duration(5) * time.Second))

		if len(agt.readBuf) < 8 {
			// 长度信息还没有读取到
			log.Debugln("准备读取报文，当前报文长度小于8")
			readBuf := make([]byte, 256)
			n, err := agt.conn.Read(readBuf)
			log.Debugln("取到报文")
			if err != nil {
				log.Errorf("读取数据时报错，报错内容: %s", err.Error())
				return nil, err
			}
			agt.conn.SetReadDeadline(time.Time{})
			log.Debugf("此次读取到了 %d 字节的数据", n)
			log.Debugf("读取到 报文，内容: %v", readBuf[:n])
			agt.readBuf = append(agt.readBuf, readBuf[:n]...)

			// 判断长度，如果达到8了则生成payload长度
			if len(agt.readBuf) >= 8 {
				agt.readMsgPayloadLth = common.GenIntFromLength(agt.readBuf[0:8])
				log.Debugf("读取到足够长度的报文，解析得到的报文长度为 %d", agt.readMsgPayloadLth)
			}
		} else if agt.readMsgPayloadLth+12 > uint64(len(agt.readBuf)) {
			log.Debugln("报文长度信息已经都读取完成，但是整个报文还没有全部传输")
			// 长度信息读取到了，但是payload没有读取完成
			readBuf := make([]byte, 256)
			n, err := agt.conn.Read(readBuf)
			if err != nil {
				log.Errorf("读取数据时报错，报错内容: %s", err.Error())
				return nil, err
			}
			agt.conn.SetReadDeadline(time.Time{})
			log.Debugln("此次读取到了 %d 字节的数据", n)
			agt.readBuf = append(agt.readBuf, readBuf[:n]...)
		} else {
			log.Debugln("报文已经都读取完成")
			agt.conn.SetReadDeadline(time.Time{})
			// 整个消息都读取到了，此时agt.readBuf中可能包含一个或一个以上的消息内容
			msgLength := 12 + agt.readMsgPayloadLth
			msgTypeBytes := agt.readBuf[8:12]
			log.Debugf("报文总长度 %d, 消息类型 %d, 消息体长度 %d", msgLength, common.GenIntFromType(msgTypeBytes), agt.readMsgPayloadLth)

			if agt.readMsgPayloadLth == 0 {
				// 不存在消息体
				if uint64(len(agt.readBuf)) == msgLength {
					agt.readBuf = []byte{}
				} else {
					agt.readBuf = agt.readBuf[msgLength:len(agt.readBuf)]
				}
				// 生成消息
				msg := &msg.Msg{
					Type:     common.GenIntFromType(msgTypeBytes),
					RawDatas: []byte{},
					Msg:      nil,
				}

				agt.readMsgPayloadLth = 0
				if len(agt.readBuf) >= 8 {
					agt.readMsgPayloadLth = common.GenIntFromLength(agt.readBuf[0:8])
					log.Debugf("读取到足够长度的报文，解析得到的报文长度为 %d, 报文长度元数据: %v,buf内容: %v", agt.readMsgPayloadLth, agt.readBuf[0:8], agt.readBuf)
				}
				return msg, nil
			} else {
				// 存在消息体
				msgPayloadBytes := agt.readBuf[12:msgLength]
				agt.readBuf = agt.readBuf[msgLength:len(agt.readBuf)]

				// 生成消息
				msg := &msg.Msg{
					Type:     common.GenIntFromType(msgTypeBytes),
					RawDatas: msgPayloadBytes,
				}

				agt.readMsgPayloadLth = 0
				if len(agt.readBuf) >= 8 {
					agt.readMsgPayloadLth = common.GenIntFromLength(agt.readBuf[0:8])
					log.Debugf("读取到足够长度的报文，解析得到的报文长度为 %d, 报文长度元数据: %v,buf内容: %v", agt.readMsgPayloadLth, agt.readBuf[0:8], agt.readBuf)
				}

				return msg, nil
			}
		}
	}
}
