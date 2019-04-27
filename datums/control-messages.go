package datums

import (
	"regexp"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type Controller struct {
	Regex *regexp.Regexp
}

type ControllerResult struct {
	Hostname    string
	Host        host.InfoStat
	CPU         cpu.InfoStat
	DiskUsage   disk.UsageStat
	DiskPart    disk.PartitionStat
	LoadAverage load.AvgStat
	MiscStat    load.MiscStat
	VirtMem     mem.VirtualMemoryStat
	SwapMem     mem.SwapMemoryStat
	Interfaces  net.InterfaceStat
}

type ControlMessage struct {
	Hosts []string
}
