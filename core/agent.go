package core

import (
	"context"
	"microagent/common"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"time"
	"sync"
	"net"
	"microagent/msg"
	//"fmt"
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
    Msg   map[string]interface{}
}

// agent结构体，插件方式，每个future 作为一个interface进行实现，future和主agent通过chan event进行
type Agent struct {
	futures map[string]Future //支持的future map
	cancel     context.CancelFunc // ctx cancel
	ctx        context.Context // ctx
	state      int // agent 状态字段
	conn       net.Conn // agent的Conn
	readBuf	   []byte // 读取的缓存
	readMsgPayloadLth uint64 // 读取的当前消息的长度
	readTotalBytesLth uint64 // 读取的总的消息长度
}

// future interface，每个future必须要实现内部的几个方法
type Future interface {
	Init() error
	Start(agtCtx context.Context, chMsg chan *InternalMsg) error
	Stop() error
	Destory() error
}

// Agent初始化
func NewAgent(sizeEvtBuf int) *Agent {
	agt := Agent{
		futures: 	map[string]Future{},
		state:      Waiting,
	}
	return &agt
}

// 注册future
func (agt *Agent) RegisterFuture(name string, fucture Future) error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.futures[name] = fucture
	return fucture.Init()
}

// 启动agent, 同时启动
func (agt *Agent) Start() error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.state = Running
	agt.ctx, agt.cancel = context.WithCancel(context.Background())
	
	return agt.startFutures()
}

// 新建一个协程，用于处理其他协程发来的消息
func (agt *Agent) FutureProcessGroutine(agtCtx context.Context,  chMsg chan *InternalMsg) {
	for {

		select {
		case fuMsg := <- chMsg:
			switch fuMsg.Msg["futname"] {
			case "Heartbeat":
				log.Infoln(*fuMsg)
			case "Collect":
				log.Infoln(*fuMsg)
			default:
				log.Errorln("[Agent]不存在的插件名, 请检查消息")
			}
		case <-agtCtx.Done():
			return
		}
	}
}

// 启动Futures
func (agt *Agent) startFutures() error {
	var msgChan = make(chan *InternalMsg, 20)
	// 启动个gorountine 读取内部插件的消息
	go agt.FutureProcessGroutine(agt.ctx, msgChan)
	
	for name, future := range agt.futures {
		log.Infof("[Agent] future: %s 准备启动....", name)
		go future.Start(agt.ctx, msgChan)
	}

	return nil
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
	for name, future := range agt.futures {
		if err = future.Stop(); err != nil {
			errs.ErrorSlice = append(errs.ErrorSlice,
				ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	}

	return ae.FutureError()
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
	for name, future := range agt.futures {
		if err = future.Destory(); err != nil {
			errs.ErrorSlice = append(errs.ErrorSlice,
				ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	}
	return ae.FutureError()
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
