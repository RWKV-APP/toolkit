package llm_toolkit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/ghw/pkg/cpu"
	"github.com/jaypipes/ghw/pkg/gpu"
	"github.com/jaypipes/ghw/pkg/memory"
	"github.com/jaypipes/ghw/pkg/product"
	gocpu "github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type HardwareInfo struct {
	Memory  *memory.Info  `json:"memory"`
	CPU     *cpu.Info     `json:"cpu"`
	GPU     *gpu.Info     `json:"gpu"`
	Product *product.Info `json:"product"`
}

func (h *HardwareInfo) JSONString() string {
	marshal, err := json.Marshal(h)
	if err != nil {
		return ""
	}
	return string(marshal)
}

type UsageInfo struct {
	CPU     CPUUsageInfo        `json:"cpu"`
	Memory  MemoryUsageInfo     `json:"memory"`
	Process *ProcessMemoryUsage `json:"process,omitempty"`
}

type CPUUsageInfo struct {
	Percent       float64 `json:"percent"`
	LogicalCores  int     `json:"logical_cores"`
	PhysicalCores int     `json:"physical_cores"`
}

type MemoryUsageInfo struct {
	Total       uint64  `json:"total"`
	Available   uint64  `json:"available"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

type ProcessMemoryUsage struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name,omitempty"`
	CPUPercent    float64 `json:"cpu_percent"`
	CPUPercentRaw float64 `json:"cpu_percent_raw"`
	RSS           uint64  `json:"rss"`
	VMS           uint64  `json:"vms"`
	HWM           uint64  `json:"hwm,omitempty"`
	Data          uint64  `json:"data,omitempty"`
	Stack         uint64  `json:"stack,omitempty"`
	Locked        uint64  `json:"locked,omitempty"`
	Swap          uint64  `json:"swap,omitempty"`
	MemoryPercent float32 `json:"memory_percent"`
}

func (u UsageInfo) JSONString() string {
	marshal, err := json.Marshal(u)
	if err != nil {
		return ""
	}
	return string(marshal)
}

func (u UsageInfo) String() string {
	return u.JSONString()
}

func GetHardwareInfo() (HardwareInfo, error) {
	memInfo, err := ghw.Memory(context.Background())
	if err != nil {
		return HardwareInfo{}, err
	}

	cpuInfo, err := ghw.CPU(context.Background())
	if err != nil {
		return HardwareInfo{}, err
	}

	productInfo, err := ghw.Product(context.Background())
	if err != nil {
		return HardwareInfo{}, err
	}

	gpuInfo, err := ghw.GPU(context.Background())
	if err != nil {
		return HardwareInfo{}, err
	}

	return HardwareInfo{
		Memory:  memInfo,
		CPU:     cpuInfo,
		GPU:     gpuInfo,
		Product: productInfo,
	}, nil
}

func GetHardwareUsageInfo() (UsageInfo, error) {
	return GetHardwareUsageInfoForPID(int32(os.Getpid()))
}

func GetHardwareUsageInfoForPID(pid int32) (UsageInfo, error) {
	cpuPercents, err := gocpu.Percent(200*time.Millisecond, false)
	if err != nil {
		return UsageInfo{}, err
	}

	logicalCores, err := gocpu.Counts(true)
	if err != nil {
		return UsageInfo{}, err
	}

	physicalCores, err := gocpu.Counts(false)
	if err != nil {
		return UsageInfo{}, err
	}

	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return UsageInfo{}, err
	}

	info := UsageInfo{
		CPU: CPUUsageInfo{
			Percent:       firstOrZero(cpuPercents),
			LogicalCores:  logicalCores,
			PhysicalCores: physicalCores,
		},
		Memory: MemoryUsageInfo{
			Total:       virtualMemory.Total,
			Available:   virtualMemory.Available,
			Used:        virtualMemory.Used,
			Free:        virtualMemory.Free,
			UsedPercent: virtualMemory.UsedPercent,
		},
	}

	if pid > 0 {
		processInfo, err := getProcessUsage(pid, logicalCores)
		if err != nil {
			return UsageInfo{}, err
		}
		info.Process = processInfo
	}

	return info, nil
}

func MonitorHardwareUsage(ctx context.Context, interval time.Duration, onUsage func(UsageInfo) error) error {
	return MonitorHardwareUsageForPID(ctx, int32(os.Getpid()), interval, onUsage)
}

func MonitorHardwareUsageForPID(ctx context.Context, pid int32, interval time.Duration, onUsage func(UsageInfo) error) error {
	if interval <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}

	emit := func() error {
		usage, err := GetHardwareUsageInfoForPID(pid)
		if err != nil {
			return err
		}
		if onUsage != nil {
			return onUsage(usage)
		}
		return nil
	}

	if err := emit(); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := emit(); err != nil {
				return err
			}
		}
	}
}

func getProcessUsage(pid int32, logicalCores int) (*ProcessMemoryUsage, error) {
	proc, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return nil, err
	}

	memPercent, err := proc.MemoryPercent()
	if err != nil {
		return nil, err
	}

	cpuPercentRaw, err := proc.Percent(200 * time.Millisecond)
	if err != nil {
		return nil, err
	}

	name, _ := proc.Name()

	cpuPercent := cpuPercentRaw
	if logicalCores > 0 {
		cpuPercent = cpuPercentRaw / float64(logicalCores)
	}

	return &ProcessMemoryUsage{
		PID:           pid,
		Name:          name,
		CPUPercent:    cpuPercent,
		CPUPercentRaw: cpuPercentRaw,
		RSS:           memInfo.RSS,
		VMS:           memInfo.VMS,
		HWM:           memInfo.HWM,
		Data:          memInfo.Data,
		Stack:         memInfo.Stack,
		Locked:        memInfo.Locked,
		Swap:          memInfo.Swap,
		MemoryPercent: memPercent,
	}, nil
}

func firstOrZero(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}
