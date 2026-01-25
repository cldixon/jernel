package metrics

import (
	"encoding/json"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// BatteryInfo holds battery status (nil on desktops or if unavailable)
type BatteryInfo struct {
	Percent  float64 `json:"percent"`
	Charging bool    `json:"charging"`
}

// LoadAverages holds Unix-style load averages (nil on Windows)
type LoadAverages struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// NetworkIO holds network traffic counters
type NetworkIO struct {
	BytesSent uint64 `json:"bytes_sent"`
	BytesRecv uint64 `json:"bytes_recv"`
}

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

	// Optional metrics (nil if unavailable on platform)
	LoadAverages *LoadAverages `json:"load_averages,omitempty"`
	SwapTotal    *uint64       `json:"swap_total,omitempty"`
	SwapUsed     *uint64       `json:"swap_used,omitempty"`
	SwapPercent  *float64      `json:"swap_percent,omitempty"`
	Battery      *BatteryInfo  `json:"battery,omitempty"`
	ProcessCount *int          `json:"process_count,omitempty"`
	NetworkIO    *NetworkIO    `json:"network_io,omitempty"`
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

// Gather collects current system metrics and returns a snapshot
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

	snapshot := &Snapshot{
		Timestamp:     time.Now(),
		Uptime:        time.Duration(uptimeSeconds) * time.Second,
		MemoryTotal:   memInfo.Total,
		MemoryUsed:    memInfo.Used,
		MemoryPercent: memInfo.UsedPercent,
		CPUPercent:    cpuPercent,
		DiskTotal:     diskInfo.Total,
		DiskUsed:      diskInfo.Used,
		DiskPercent:   diskInfo.UsedPercent,
	}

	// Collect optional metrics (failures are silently ignored)
	gatherOptionalMetrics(snapshot, memInfo)

	return snapshot, nil
}

// gatherOptionalMetrics collects platform-specific metrics that may not be available
func gatherOptionalMetrics(snapshot *Snapshot, memInfo *mem.VirtualMemoryStat) {
	// Load averages (not available on Windows)
	if runtime.GOOS != "windows" {
		if loadInfo, err := load.Avg(); err == nil {
			snapshot.LoadAverages = &LoadAverages{
				Load1:  loadInfo.Load1,
				Load5:  loadInfo.Load5,
				Load15: loadInfo.Load15,
			}
		}
	}

	// Swap memory
	if swapInfo, err := mem.SwapMemory(); err == nil && swapInfo.Total > 0 {
		snapshot.SwapTotal = &swapInfo.Total
		snapshot.SwapUsed = &swapInfo.Used
		snapshot.SwapPercent = &swapInfo.UsedPercent
	}

	// Process count
	if pids, err := process.Pids(); err == nil {
		count := len(pids)
		snapshot.ProcessCount = &count
	}

	// Network I/O (aggregate across all interfaces)
	if netIO, err := net.IOCounters(false); err == nil && len(netIO) > 0 {
		snapshot.NetworkIO = &NetworkIO{
			BytesSent: netIO[0].BytesSent,
			BytesRecv: netIO[0].BytesRecv,
		}
	}

	// Battery (laptops only)
	gatherBatteryInfo(snapshot)
}

// gatherBatteryInfo attempts to get battery status
// This is platform-specific and may not be available on all systems
func gatherBatteryInfo(snapshot *Snapshot) {
	// gopsutil doesn't have a direct battery API, so we use host.SensorsTemperatures
	// as a proxy to detect if we're on a system that might have battery
	// For now, we'll skip battery collection - it requires platform-specific code
	// TODO: Implement battery collection using platform-specific APIs or external packages
	//
	// On macOS: ioreg -l | grep -i capacity
	// On Linux: /sys/class/power_supply/BAT*/capacity
	// On Windows: WMI queries
}
