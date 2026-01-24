package metrics

import (
	"encoding/json"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// Snapshot represents a moment-in-time capture of system state
type Snapshot struct {
	Timestamp     time.Time     `json:"timestamp"`
	Uptime        time.Duration `json:"uptime"`
	MemoryTotal   uint64        `json:"memory_total"`
	MemoryUsed    uint64        `json:"memory_used"`
	MemoryPercent float64       `json:"memory_percent"`
	CPUPercent    float64       `json:"cpu_percent"`
	DiskTotal     uint64        `json:"disk_total"`
	DiskUsed      uint64        `json:"disk_used"`
	DiskPercent   float64       `json:"disk_percent"`
}

// ToJSON serializes the snapshot to a JSON string
func (s *Snapshot) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SnapshotFromJSON deserializes a snapshot from a JSON string
func SnapshotFromJSON(data string) (*Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return nil, err
	}
	return &s, nil
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
