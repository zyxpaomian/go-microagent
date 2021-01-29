package update

import (
	"context"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"microagent/core"
	"sync"
	"time"
)

type UpdateController struct {
	agtCtx   context.Context
	stopChan chan struct{}
	name     string
	updateMgt *UpdateOper
}

func NewUpdateController(name string) *UpdateController {
	return &UpdateController{
		stopChan: make(chan struct{}),
		name:     name,
	}
}

func (u *UpdateController) Init() error {
	log.Infoln("[Update] 初始化Update的goroutine....", u.name)
	u.updateMgt = NewUpdate()
	return nil
}

func (u *UpdateController) Start(agtCtx context.Context, chMsg chan *core.InternalMsg) error {
	log.Infoln("[Update] 启动Update的goroutine....", u.name)
	internalMsg := &core.InternalMsg{
		Lock: new(sync.RWMutex),
		Msg:  make(map[string]interface{}),
	}
	for {
		u.updateMgt.CheckUpdate()
		select {
		case <-agtCtx.Done():
			u.stopChan <- struct{}{}
			break
		default:
			time.Sleep(time.Millisecond * 30000)
			// 并发安全，加个锁
			internalMsg.Lock.Lock()
			internalMsg.Msg["futname"] = "Update"
			internalMsg.Msg["updateswitch"] = u.updateMgt.UpdateSwitch
			internalMsg.Lock.Unlock()

			chMsg <- internalMsg
		}
	}
	return ae.New("[Update] Update 启动失败")
}

func (u *UpdateController) Stop() error {
	log.Infoln("[Update] 关闭Update的goroutine....", u.name)
	select {
	case <-u.stopChan:
		return nil
	case <-time.After(time.Second * 1):
		return ae.New("[Update] 关闭Update goroutine 超时")
	}
}

func (u *UpdateController) Destory() error {
	log.Infoln("[Update] 销毁Update的goroutine....", u.name)
	return nil
}
