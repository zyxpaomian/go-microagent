package collect

import (
	"context"
	ae "microagent/common/error"
    "microagent/core"
	"fmt"
	"time"
)

type Collector struct {
	evtReceiver core.EventReceiver
	agtCtx      context.Context
	stopChan    chan struct{}
	name        string
	subname		string
	content     string
	ColEvent	*ColItem

}

func NewCollector(name string, content string) *Collector {
	return &Collector{
		stopChan: make(chan struct{}),
		name:     name,
		content:  content,
	}
}

func (c *Collector) Init(evtReceiver core.EventReceiver) error {
	fmt.Println("initialize Collector", c.name)
	c.evtReceiver = evtReceiver
	c.ColEvent = NewCol()
	return nil
}

 
func (c *Collector) Start(agtCtx context.Context) error {
	fmt.Println("start Collector", c.name)
	for {
		c.ColEvent.CollectRun()
		c.content = c.ColEvent.Uptime
		select {
		case <-agtCtx.Done():
			c.stopChan <- struct{}{}
			break
		default:
			time.Sleep(time.Millisecond * 50000)
			c.evtReceiver.OnEvent(core.Event{c.name, c.subname, c.content})
		}
	}
}

func (c *Collector) Stop() error {
	fmt.Println("stop Collector", c.name)
	select {
	case <-c.stopChan:
		return nil
	case <-time.After(time.Second * 1):
		return ae.New("[Collector] 关闭Collector groutine 超时")
	}
}

func (c *Collector) Destory() error {
	fmt.Println(c.name, "released resources.")
	return nil
}
