package core

import (
	"context"
	ae "microagent/common/error"
	"fmt"
	"strings"
	"sync"
)

const (
	Waiting = iota
	Running
)


type CollectorsError struct {
	CollectorErrors []error
}

func (ce CollectorsError) Error() string {
	var strs []string
	for _, err := range ce.CollectorErrors {
		strs = append(strs, err.Error())
	}
	return strings.Join(strs, ";")
}

//  agent结构体，插件方式，每个future 作为一个interface进行实现，future和主agent通过chan event进行
type Agent struct {
	Futures map[string]Future
	evtBuf     chan Event
	cancel     context.CancelFunc
	ctx        context.Context
	state      int
}

// future interface，每个future必须要实现内部的几个方法
type Future interface {
	Init(evtReceiver EventReceiver) error
	Start(agtCtx context.Context) error
	Stop() error
	Destory() error
}

// 各个插件通信使用的结构体
type Event struct {
	Futname  string // future名称
    Subname string // future明细内容
	Content string // 通信内容
}

// 通信接收接口
type EventReceiver interface {
	OnEvent(evt Event)
}

// 该方法将各future 数据吐到agent的buf chan 上
func (agt *Agent) OnEvent(evt Event) {
	agt.evtBuf <- evt
}

// Agent 初始化
func NewAgent(sizeEvtBuf int) *Agent {
	agt := Agent{
		Futures: map[string]Future{},
		evtBuf:     make(chan Event, sizeEvtBuf),
		state:      Waiting,
	}
	return &agt
}

// 注册future
func (agt *Agent) RegisterFuture(name string, fucture Future) error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.Futures[name] = fucture
	return fucture.Init(agt)
}

// 启动agent, 同时启动
func (agt *Agent) Start() error {
	if agt.state != Waiting {
		return ae.StateError()
	}
	agt.state = Running

	agt.ctx, agt.cancel = context.WithCancel(context.Background())
	go agt.FutureProcessGroutine()
	return agt.startFutures()
}

// 进程消息队列，每收集10个消息上报一次
func (agt *Agent) FutureProcessGroutine() {
	//var evtSeg [10]Event
    var evtSeg Event
	for {
		select {
		case evtSeg = <-agt.evtBuf:
            switch evtSeg.Futname {
            case "collect":
                fmt.Println(evtSeg.Subname)
            }
		case <-agt.ctx.Done():
				return
		}
	}
}

// 启动Futures
func (agt *Agent) startFutures() error {
	var err error
	var errs CollectorsError
	var mutex sync.Mutex

	for name, future := range agt.Futures {
		go func(name string, future Future, ctx context.Context) {
			defer func() {
				mutex.Unlock()
			}()
            mutex.Lock()
			err = future.Start(ctx)

			if err != nil {
				errs.CollectorErrors = append(errs.CollectorErrors,
					ae.New(name+":"+err.Error()))
			}
		}(name, future, agt.ctx)
	}
	if len(errs.CollectorErrors) == 0 {
		return nil
	}
	return errs
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
	var errs CollectorsError
	for name, future := range agt.Futures {
		if err = future.Stop(); err != nil {
			errs.CollectorErrors = append(errs.CollectorErrors,
				ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.CollectorErrors) == 0 {
		return nil
	}

	return errs
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
	var errs CollectorsError
	for name, future := range agt.Futures {
		if err = future.Destory(); err != nil {
			errs.CollectorErrors = append(errs.CollectorErrors,
				ae.New(name+":"+err.Error()))
		}
	}
	if len(errs.CollectorErrors) == 0 {
		return nil
	}
	return errs
}