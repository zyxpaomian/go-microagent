package collect

import (
	"runtime"
	"time"
)

var Col *ColItem

type ColItem struct {
	Uptime   string
	CpuArch  string
	CpuNum   int32
	MemTotal string
	ColTime  string
}


type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// Agent 初始化
func NewCol() *ColItem {
	col := ColItem{
		ColTime: time.Now().Format("2006/1/2 15:04:05"),
	}
	return &col
}

func (co *ColItem) CollectRun() {
	co.GetCpuArch()
	co.GetCpuNum()
	co.ColTime = time.Now().Format("2006/1/2 15:04:05")
}

// cpu 架构
func (co *ColItem) GetCpuArch() {
	co.CpuArch = runtime.GOARCH
}

// cpu数量
func (co *ColItem) GetCpuNum() {
	co.CpuNum = int32(runtime.NumCPU())
}