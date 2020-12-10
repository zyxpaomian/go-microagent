package collect

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var Col *ColItem

type ColItem struct {
	Uptime   string
	CpuArch  string
	CpuNum   int
	MemTotal string
	ColTime  string
}

var (
	kernel = syscall.NewLazyDLL("Kernel32.dll")
)

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
	co.GetUptime()
	co.GetCpuArch()
	co.GetCpuNum()
	co.GetMemory()
	co.ColTime = time.Now().Format("2006/1/2 15:04:05")
}

// 开机时间
func (co *ColItem) GetUptime() {
	GetTickCount := kernel.NewProc("GetTickCount")
	r, _, _ := GetTickCount.Call()
	if r == 0 {
		co.Uptime = "-1s"
	}
	ms := time.Duration(r * 1000 * 1000)
	co.Uptime = ms.String()
}

// cpu 架构
func (co *ColItem) GetCpuArch() {
	co.CpuArch = runtime.GOARCH
}

// cpu数量
func (co *ColItem) GetCpuNum() {
	co.CpuNum = runtime.NumCPU()
}

// 内存总量
func (co *ColItem) GetMemory() {
	GlobalMemoryStatusEx := kernel.NewProc("GlobalMemoryStatusEx")
	var memInfo memoryStatusEx
	memInfo.cbSize = uint32(unsafe.Sizeof(memInfo))
	mem, _, _ := GlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&memInfo)))
	if mem == 0 {
		co.MemTotal = "-1"
	}

	co.MemTotal = fmt.Sprint(memInfo.ullTotalPhys / (1024 * 1024))
}
