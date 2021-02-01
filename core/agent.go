package core

import (
	"bytes"
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"microagent/common"
	"microagent/controller"
	"microagent/task/update"
	cfg "microagent/common/configparse"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"microagent/msg"
	"net"
	"sync"
	"time"
	//"os/exec"
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
	sendLock          *sync.Mutex        // 发送锁
	readBuf           []byte             // 读取的缓存
	readMsgPayloadLth uint64             // 读取的当前消息的长度
	readTotalBytesLth uint64             // 读取的总的消息长度
	msgChan	chan *InternalMsg	// 内部通信的channel
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
	chMsg := make(chan *InternalMsg, 20)
	agt := Agent{
		futures:           map[string]Future{},
		state:             Waiting,
		conn:              nil,
		sendLock:          &sync.Mutex{},
		readBuf:           []byte{},
		readMsgPayloadLth: 0,
		readTotalBytesLth: 0,
		msgChan: chMsg,
	}
	return &agt
}

// Agent初始化
func (agt *Agent) reset() {
	log.Infoln("[Agent] 重置Agent 对象....")
	// 重置Agent 对象， future 不要重置，避免还要再次注册
	agt.state = Waiting
	agt.conn = nil
	agt.sendLock = &sync.Mutex{}
	agt.readBuf = []byte{}
	agt.readMsgPayloadLth = 0
	agt.readTotalBytesLth = 0
	agt.msgChan = nil


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
	for {
		log.Infoln("[Agent] 准备启动Agent....")
		agt.Start()
		log.Errorln("[Agent] 启动Agent失败，等待10S 后进行重新启动")
		agt.Stop()
		agt.reset()
		time.Sleep(time.Duration(10) * time.Second)
	}
}

// 启动agent, 同时启动所有插件
func (agt *Agent) Start() {
	if agt.state != Waiting {
		return
	}
	agt.state = Running

	agt.ctx, agt.cancel = context.WithCancel(context.Background())

	svrAddr, err := controller.Commctrl.ElectServer()
	if err != nil {
		log.Errorf("[Agent] 获取Agent链接信息失败, 请检查配置, 错误信息: %s", err.Error())
		return
	}

	// 启动tcp 服务，和server 端的主要通信方式
	conn, err := net.Dial("tcp", svrAddr)
	if err != nil {
		log.Errorf("[Agent] 无法连接到服务器 %s，报错信息 %s", svrAddr, err.Error())
		return
	}
	defer conn.Close()
	agt.conn = conn


	// 启动各个插件
	if err = agt.startFutures(); err != nil {
		return
	}

	var wg sync.WaitGroup

	// 用于监听服务端发来的消息
	wg.Add(1)
	go agt.listen(&wg)

	// 用于发送心跳包, 核心功能，不做插件处理
	wg.Add(1)
	go agt.heartbeat(&wg)

	// 等待线程运行结束
	wg.Wait()
}

// 新建一个协程，用于处理其他协程发来的消息
func (agt *Agent) fuMsgHandle(agtCtx context.Context) {
	for {
		if agt.state == Erroring {
			log.Errorln("[Agent] Agent状态错误，跳出循环，等待Agent 自动重置")
			break
		}
		select {
		case fuMsg := <-agt.msgChan:
			switch fuMsg.Msg["futname"] {
			case "Collect":
				// 生成收集包
				collectMsg := &msg.Msg{
					Type: msg.CLIENT_MSG_COLLECT,
					Msg: &msg.Collect{
						Cpuarch:  fuMsg.Msg["cpuarch"].(string),
						Cpunum:   fuMsg.Msg["cpunum"].(int32),
						ColTime:  fuMsg.Msg["coltime"].(string),
					},
				}
				// 发送收集项报文
				agt.sendMsg(collectMsg)
			case "Rpms":
				// 生成RPM信息
				rpmMsg := &msg.Msg{
					Type: msg.CLIENT_MSG_RPMS,
					Msg: &msg.Rpms{
						Rpmlist:   fuMsg.Msg["rpmlist"].([]string),
					},
				}
				// 发送RPM
				agt.sendMsg(rpmMsg)
			/*case "Update":
				updateSwitch := fuMsg.Msg["updateswitch"].(bool)
				if updateSwitch == true {
					log.Infoln("需要重启服务")
					cmd := exec.Command("bash", "-c", "./bin/restart.sh")
					output, _ := cmd.CombinedOutput()
					log.Infoln(string(output))
					//syscall.Kill(os.Getpid(), syscall.SIGHUP) 
				}*/
			default:
				log.Errorln("[Agent]不存在的插件名, 请检查内部消息")
			}
		case <-agtCtx.Done():
			break
		}
	}
}


// 启动Futures
func (agt *Agent) startFutures() error {
	var err error
	var errs FutureErrors
	var errInfo string

	// 启动个gorountine 读取内部插件的消息
	go agt.fuMsgHandle(agt.ctx)

	for name, future := range agt.futures {
		// 顺序启动go routine， 确保每个插件第一次都能正确执行
		go func(name string, future Future, ctx context.Context, chMsg chan *InternalMsg) {
			log.Infof("[Agent] future: %s 准备启动....", name)
			if err = future.Start(agt.ctx, agt.msgChan); err != nil {
				errs.ErrorSlice = append(errs.ErrorSlice, ae.New(name+":"+err.Error()))
			}
		}(name, future, agt.ctx, agt.msgChan)
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	} else {
		for _, er := range errs.ErrorSlice {
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
			errs.ErrorSlice = append(errs.ErrorSlice, ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.ErrorSlice) == 0 {
		return nil
	} else {
		for _, er := range errs.ErrorSlice {
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
		for _, er := range errs.ErrorSlice {
			erInfo := fmt.Sprintf("[Agent] 插件销毁错误, %v,", er.Error())
			errInfo = errInfo + erInfo
		}
		return ae.New(errInfo)
	}
}

// 心跳服务，基础功能，不当作插件添加
func (agt *Agent) heartbeat(wg *sync.WaitGroup) {
	log.Infoln("[Agent] 开启心跳包发送线程....")
	for {
		if agt.state == Erroring {
			log.Errorln("[Agent] Agent状态错误，跳出循环，等待Agent 自动重置")
			break
		}

		// 生成心跳包
		heartbeatMsg := &msg.Msg{
			Type: msg.CLIENT_MSG_HEARTBEAT,
			Msg: &msg.Heartbeat{
				Status:        "GOOD",
				HeartbeatTime: time.Now().Format("2006-01-02 15:04:05"),
			},
		}
		// 发送心跳包
		agt.sendMsg(heartbeatMsg)

		// sleep 10秒
		time.Sleep(time.Duration(10) * time.Second)
	}
	wg.Done()
}

func (agt *Agent) handleHeartbeatMsg(serverMsg *msg.Msg) {
	log.Infoln("[Agent] 收到了服务端发送来的Heartbeat 应答包")
}

func (agt *Agent) handleAgentUpdateMsg(serverMsg *msg.Msg) {
	log.Infoln("[Agent] 收到了服务端发送来的客户端升级包, 准备升级")
	update.Update.UpdateInit()
	update.Update.GoUpdate()

	
}

/* 和server 端通信主要使用的方法 */
// 监听tcp 长链接
func (agt *Agent) listen(wg *sync.WaitGroup) {
	log.Infoln("[Agent] 开启TCP监听线程....")
	for {
		if agt.state == Erroring {
			log.Errorln("[Agent] Agent状态错误，跳出循环，等待Agent 自动重置")
			break
		}
		msg, err := agt.getMsg()
		if err != nil {
			log.Errorf("[Agent]从服务端读取消息失败，结束与该服务器的连接，报错内容: %s", err.Error())
			break
		}
		// 处理消息
		agt.handleMsg(msg)

	}
	wg.Done()
}

// 处理TCP 收到的消息
func (agt *Agent) handleMsg(serverMsg *msg.Msg) {
	log.Infof("[Agent] 收到了服务器端的消息请求，请求类型: %d", serverMsg.Type)
	switch serverMsg.Type {
	case msg.SERVER_MSG_HEARTBEAT_RESPONSE:
		agt.handleHeartbeatMsg(serverMsg)
	case msg.SERVER_MSG_AGENT_UPDATE:
		agt.handleAgentUpdateMsg(serverMsg)		
	default:
		log.Errorf("[Agent] 未知的消息类型 %d", serverMsg.Type)
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
			log.Errorf("[Agent] protobuf消息生成失败: %s", err.Error())
			// Agent 错误，等待重载
			agt.state = Erroring
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

	log.Debugf("[Agent] 发送报文，报文长度 %d，类型 %d，消息体长度 %d", msgSize+12, msgType, msgSize)
	log.Debugf("[Agent] 报文内容: %v", packet)

	agt.sendLock.Lock()
	// 设置写入超时
	agt.conn.SetWriteDeadline(time.Now().Add(time.Duration(cfg.GlobalConf.GetInt("common", "sendwrtimeout")) * time.Second))
	_, err = agt.conn.Write(packet)
	if err != nil {
		log.Errorf("[Agent] sendMsg失败: %s", err.Error())
		// Agent 错误，等待重载
		agt.state = Erroring
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
		agt.conn.SetReadDeadline(time.Now().Add(time.Duration(20) * time.Second))

		if len(agt.readBuf) < 8 {
			// 长度信息还没有读取到
			log.Debugln("[Agent] 准备读取报文，当前报文长度小于8")
			readBuf := make([]byte, 512)
			n, err := agt.conn.Read(readBuf)
			log.Debugln("[Agent] 取到报文")
			if err != nil {
				log.Errorf("[Agent] 读取数据时报错，报错内容: %s", err.Error())
				return nil, err
			}
			agt.conn.SetReadDeadline(time.Time{})
			log.Debugf("[Agent] 此次读取到了 %d 字节的数据", n)
			log.Debugf("[Agent] 读取到 报文，内容: %v", readBuf[:n])
			agt.readBuf = append(agt.readBuf, readBuf[:n]...)

			// 判断长度，如果达到8了则生成payload长度
			if len(agt.readBuf) >= 8 {
				agt.readMsgPayloadLth = common.GenIntFromLength(agt.readBuf[0:8])
				log.Debugf("[Agent] 读取到足够长度的报文，解析得到的报文长度为 %d", agt.readMsgPayloadLth)
			}
		} else if agt.readMsgPayloadLth+12 > uint64(len(agt.readBuf)) {
			log.Debugln("[Agent] 报文长度信息已经都读取完成，但是整个报文还没有全部传输")
			// 长度信息读取到了，但是payload没有读取完成
			readBuf := make([]byte, 256)
			n, err := agt.conn.Read(readBuf)
			if err != nil {
				log.Errorf("[Agent] 读取数据时报错，报错内容: %s", err.Error())
				return nil, err
			}
			agt.conn.SetReadDeadline(time.Time{})
			log.Debugln("[Agent] 此次读取到了 %d 字节的数据", n)
			agt.readBuf = append(agt.readBuf, readBuf[:n]...)
		} else {
			log.Debugln("[Agent] 报文已经都读取完成")
			agt.conn.SetReadDeadline(time.Time{})
			// 整个消息都读取到了，此时agt.readBuf中可能包含一个或一个以上的消息内容
			msgLength := 12 + agt.readMsgPayloadLth
			msgTypeBytes := agt.readBuf[8:12]
			log.Debugf("[Agent] 报文总长度 %d, 消息类型 %d, 消息体长度 %d", msgLength, common.GenIntFromType(msgTypeBytes), agt.readMsgPayloadLth)

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
					log.Debugf("[Agent] 读取到足够长度的报文，解析得到的报文长度为 %d, 报文长度元数据: %v,buf内容: %v", agt.readMsgPayloadLth, agt.readBuf[0:8], agt.readBuf)
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
					log.Debugf("[Agent] 读取到足够长度的报文，解析得到的报文长度为 %d, 报文长度元数据: %v,buf内容: %v", agt.readMsgPayloadLth, agt.readBuf[0:8], agt.readBuf)
				}

				return msg, nil
			}
		}
	}
}
