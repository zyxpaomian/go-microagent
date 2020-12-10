package collect

import (
	"context"
	ae "microagent/common/error"
	log "microagent/common/formatlog"
	"microagent/core"
	"sync"
	"time"
)

type Collector struct {
	agtCtx   context.Context
	stopChan chan struct{}
	name     string
	colEvent *ColItem
}

func NewCollector(name string) *Collector {
	return &Collector{
		stopChan: make(chan struct{}),
		name:     name,
	}
}

func (c *Collector) Init() error {
	log.Infoln("[Collect]初始化Colletcor的goroutine....", c.name)
	c.colEvent = NewCol()
	return nil
}

func (c *Collector) Start(agtCtx context.Context, chMsg chan *core.InternalMsg) error {
	log.Infoln("[Collect]启动Colletcor的goroutine....", c.name)
	internalMsg := &core.InternalMsg{
		Lock: new(sync.RWMutex),
		Msg:  make(map[string]interface{}),
	}
	for {
		c.colEvent.CollectRun()
		//fmt.Println(c.content)
		select {
		case <-agtCtx.Done():
			c.stopChan <- struct{}{}
			break
		default:
			time.Sleep(time.Millisecond * 10000)
			// 并发安全，加个锁
			internalMsg.Lock.Lock()
			internalMsg.Msg["futname"] = "Collect"
			internalMsg.Msg["uptime"] = c.colEvent.Uptime
			internalMsg.Msg["cpuarch"] = c.colEvent.CpuArch
			internalMsg.Msg["cpunum"] = c.colEvent.CpuNum
			internalMsg.Msg["memtotal"] = c.colEvent.MemTotal
			internalMsg.Msg["coltime"] = c.colEvent.ColTime
			internalMsg.Lock.Unlock()

			chMsg <- internalMsg
		}
	}
	return ae.New("[Collect] Collect 启动失败")
}

func (c *Collector) Stop() error {
	log.Infoln("[Collect]关闭Colletcor的goroutine....", c.name)
	select {
	case <-c.stopChan:
		return nil
	case <-time.After(time.Second * 1):
		return ae.New("[Collect] 关闭Collect goroutine 超时")
	}
}

func (c *Collector) Destory() error {
	log.Infoln("[Collect]销毁Colletcor的goroutine....", c.name)
	return nil
}
