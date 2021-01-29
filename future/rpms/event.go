package rpms

import (
	"context"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"microagent/core"
	"sync"
	"time"
)

type RpmController struct {
	agtCtx   context.Context
	stopChan chan struct{}
	name     string
	rpmMgt *RpmOper
}

func NewRpmController(name string) *RpmController {
	return &RpmController{
		stopChan: make(chan struct{}),
		name:     name,
	}
}

func (r *RpmController) Init() error {
	log.Infoln("[Rpms] 初始化Rpms的goroutine....", r.name)
	r.rpmMgt = NewRpm()
	return nil
}

func (r *RpmController) Start(agtCtx context.Context, chMsg chan *core.InternalMsg) error {
	log.Infoln("[Rpms] 启动Rpms的goroutine....", r.name)
	internalMsg := &core.InternalMsg{
		Lock: new(sync.RWMutex),
		Msg:  make(map[string]interface{}),
	}
	for {
		r.rpmMgt.GetAllRpms()
		select {
		case <-agtCtx.Done():
			r.stopChan <- struct{}{}
			break
		default:
			time.Sleep(time.Millisecond * 30000)
			// 并发安全，加个锁
			internalMsg.Lock.Lock()
			internalMsg.Msg["futname"] = "Rpms"
			internalMsg.Msg["rpmlist"] = r.rpmMgt.RpmList
			internalMsg.Lock.Unlock()

			chMsg <- internalMsg
		}
	}
	return ae.New("[Rpms] Rpms 启动失败")
}

func (r *RpmController) Stop() error {
	log.Infoln("[Rpms] 关闭Rpms的goroutine....", r.name)
	select {
	case <-r.stopChan:
		return nil
	case <-time.After(time.Second * 1):
		return ae.New("[Rpms] 关闭Rpms goroutine 超时")
	}
}

func (r *RpmController) Destory() error {
	log.Infoln("[Rpms] 销毁Rpms的goroutine....", r.name)
	return nil
}
