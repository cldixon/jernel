package metrics

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// snapshot represents a moment-in-time capture of system state
type Snapshot struct {
	Timestamp     time.Time
	Uptime        time.Duration
	MemoryTotal   uint64
	MemoryUsed    uint64
	MemoryPercent float64
	CPUPercent    float64
	DiskTotal     uint64
	DiskUsed      uint64
	DiskPercent   float64
}

// gather collects current system metrics and returns a snapshot
func Gather() (*Snapshot, error) {
	// get memory stats
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	// get uptime
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		return nil, err
	}

	// get cpu usage (average across all cores, 1 second sample)
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}

	cpuPercent := 0.0
	if len(cpuPercents) > 0 {
		cpuPercent = cpuPercents[0]
	}

	// get disk usage for root partition
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Timestamp:     time.Now(),
		Uptime:        time.Duration(uptimeSeconds) * time.Second,
		MemoryTotal:   memInfo.Total,
		MemoryUsed:    memInfo.Used,
		MemoryPercent: memInfo.UsedPercent,
		CPUPercent:    cpuPercent,
		DiskTotal:     diskInfo.Total,
		DiskUsed:      diskInfo.Used,
		DiskPercent:   diskInfo.UsedPercent,
	}, nil
}
