package main

import (
    "microagent/core"
    "microagent/future/collect"
    "fmt"
)
func main() {
    agt := core.NewAgent(20) 
    collector := collect.NewCollector("collect", "collect")
	agt.RegisterFuture("collect", collector)
	if err := agt.Start(); err != nil {
		fmt.Printf("start error %v\n", err)
	}
    select {}
	//fmt.Println(agt.Start())
	//time.Sleep(time.Second * 1)
	//agt.Stop()
	//agt.Destory()
}