package datums

import (
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type ControllerReq struct {
	Pattern string
}

type ControllerResult struct {
	Hostname    string
	Host        host.InfoStat
	CPU         []cpu.InfoStat
	DiskUsage   disk.UsageStat
	DiskPart    []disk.PartitionStat
	LoadAverage load.AvgStat
	MiscStat    load.MiscStat
	VirtMem     mem.VirtualMemoryStat
	SwapMem     mem.SwapMemoryStat
	Interfaces  []net.InterfaceStat
}

type ControlMessage struct {
	Hosts []string
}

func (sysinfo *ControllerResult) GetSystemInfo() error {
	hostInfo, err := host.Info()
	if err != nil {
		return err
	}
	sysinfo.Host = *hostInfo
	cpuInfo, err := cpu.Info()
	if err != nil {
		return err
	}
	sysinfo.CPU = cpuInfo
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return err
	}
	sysinfo.DiskUsage = *diskInfo
	diskPartInfo, err := disk.Partitions(true)
	if err != nil {
		return err
	}
	sysinfo.DiskPart = diskPartInfo
	loadInfo, err := load.Avg()
	if err != nil {
		return err
	}
	sysinfo.LoadAverage = *loadInfo
	miscInfo, err := load.Misc()
	if err != nil {
		return err
	}
	sysinfo.MiscStat = *miscInfo
	virtMemInfo, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	sysinfo.VirtMem = *virtMemInfo
	swapInfo, err := mem.SwapMemory()
	if err != nil {
		return err
	}
	sysinfo.SwapMem = *swapInfo
	ifacesInfo, err := net.Interfaces()
	if err != nil {
		return err
	}
	sysinfo.Interfaces = ifacesInfo
	return nil
}
