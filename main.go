package main

import (
	//"fmt"
	"math/rand"
	cfg "microagent/common/configparse"
	log "microagent/common/formatlog"
	"microagent/core"
	"microagent/future/collect"
	"microagent/future/heartbeat"
	"runtime"
	"time"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())
	
	// 初始化配置 && 日志
	cfg.GlobalConf.CfgInit("./conf/microagent.ini")
	logname := cfg.GlobalConf.GetStr("common", "logname")
	log.InitLog(logname, "INFO")

	// 初始化Agent
	agt := core.NewAgent()

	// 初始化插件
	collector := collect.NewCollector("collect")
	heartbeat := heartbeat.NewHeartBeater("heartbeat")

	// 注册插件
	agt.RegisterFuture("collect", collector)
	agt.RegisterFuture("heartbeat", heartbeat)

	// 启动Agent
	go agt.Run()
	select {}
}
