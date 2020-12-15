package main

import (
	//"fmt"
	"math/rand"
	cfg "microagent/common/configparse"
	log "microagent/common/formatlog"
	"microagent/core"
	"microagent/future/collect"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())

	// 初始化配置 && 日志
	cfg.GlobalConf.CfgInit("./conf/microagent.ini")
	logname := cfg.GlobalConf.GetStr("common", "logname")
    loglevel := cfg.GlobalConf.GetStr("common", "loglevel")
	log.InitLog(logname, loglevel)	



	// 初始化Agent
	agt := core.NewAgent()

	// 初始化插件
	collector := collect.NewCollector("collect")

	// 注册插件
	agt.RegisterFuture("collect", collector)

	// 启动Agent
	go agt.Run()

	select {}
}
