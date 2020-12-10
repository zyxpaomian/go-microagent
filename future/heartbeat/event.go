package heartbeat

import (
	"context"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"microagent/core"
	"sync"
	"time"
)

type Heartbeat struct {
	agtCtx   context.Context
	stopChan chan struct{}
	name     string
}

func NewHeartBeater(name string) *Heartbeat {
	return &Heartbeat{
		stopChan: make(chan struct{}),
		name:     name,
	}
}

func (h *Heartbeat) Init() error {
	log.Infoln("[Heartbeat]初始化Heartbeat的goroutine....", h.name)
	return nil
}

func (h *Heartbeat) Start(agtCtx context.Context, chMsg chan *core.InternalMsg) error {
	log.Infoln("[Heartbeat]启动Heartbeat的goroutine....", h.name)
	internalMsg := &core.InternalMsg{
		Lock: new(sync.RWMutex),
		Msg:  make(map[string]interface{}),
	}
	for {
		select {
		case <-agtCtx.Done():
			h.stopChan <- struct{}{}
			break
		default:
			time.Sleep(time.Millisecond * 2000)
			time.Sleep(time.Duration(10) * time.Second)

			// 并发安全，加个锁
			internalMsg.Lock.Lock()
			internalMsg.Msg["futname"] = "Heartbeat"
			internalMsg.Msg["status"] = "GOOD"
			internalMsg.Msg["heartbeatTime"] = time.Now().Format("2006/1/2 15:04:05")
			internalMsg.Lock.Unlock()

			chMsg <- internalMsg
		}
	}
	return ae.New("[Heartbeat] HeartBeat 启动失败")
}

func (h *Heartbeat) Stop() error {
	log.Infoln("[Heartbeat]关闭Heartbeat的goroutine....", h.name)
	select {
	case <-h.stopChan:
		return nil
	case <-time.After(time.Second * 1):
		return ae.New("[Heartbeat] 关闭Heartbeat goroutine 超时")
	}
}

func (h *Heartbeat) Destory() error {
	log.Infoln("[Heartbeat]销毁Heartbeat的goroutine....", h.name)
	return nil
}
