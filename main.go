package main

import (
    "microagent/core"
    "microagent/future/collect"
	"microagent/future/heartbeat"
	log "microagent/common/formatlog"
	cfg "microagent/common/configparse"
    "fmt"

		"math/rand"
	"runtime"
	"time"
	
)
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UTC().UnixNano())
	cfg.GlobalConf.CfgInit("./conf/microagent.ini")
	logname := cfg.GlobalConf.GetStr("common","logname")
	log.InitLog(logname, "INFO")
    agt := core.NewAgent(20) 

    collector := collect.NewCollector("collect")
	heartbeat := heartbeat.NewHeartBeater("heartbeat")
	agt.RegisterFuture("collect", collector)
	agt.RegisterFuture("heartbeat", heartbeat)
	if err := agt.Start(); err != nil {
		fmt.Printf("start error %v\n", err)
	}

	time.Sleep(10 * time.Second)
	agt.Stop()
    //select {}
}