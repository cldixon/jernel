package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

// MachineType represents the type of machine
type MachineType string

const (
	MachineTypeLaptop    MachineType = "laptop"
	MachineTypeDesktop   MachineType = "desktop"
	MachineTypeServer    MachineType = "server"
	MachineTypeVM        MachineType = "virtual_machine"
	MachineTypeContainer MachineType = "container"
	MachineTypeUnknown   MachineType = "unknown"
)

// TimeOfDay represents the general time period
type TimeOfDay string

const (
	TimeOfDayNight     TimeOfDay = "night"     // 12am - 6am
	TimeOfDayMorning   TimeOfDay = "morning"   // 6am - 12pm
	TimeOfDayAfternoon TimeOfDay = "afternoon" // 12pm - 6pm
	TimeOfDayEvening   TimeOfDay = "evening"   // 6pm - 12am
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

// ThermalInfo holds temperature readings
type ThermalInfo struct {
	CPUTemp     *float64 `json:"cpu_temp,omitempty"` // Celsius
	GPUTemp     *float64 `json:"gpu_temp,omitempty"` // Celsius
	HighestTemp float64  `json:"highest_temp"`       // Highest sensor reading
	SensorCount int      `json:"sensor_count"`       // Number of sensors read
}

// FanInfo holds fan speed readings
type FanInfo struct {
	Speed float64 `json:"speed"` // RPM
	Name  string  `json:"name"`  // Fan identifier
}

// GPUInfo holds GPU metrics
type GPUInfo struct {
	Usage       *float64 `json:"usage,omitempty"`        // GPU utilization percent
	MemoryUsed  *uint64  `json:"memory_used,omitempty"`  // VRAM used in bytes
	MemoryTotal *uint64  `json:"memory_total,omitempty"` // Total VRAM in bytes
}

// PlatformInfo holds OS and architecture info
type PlatformInfo struct {
	OS           string `json:"os"`           // darwin, linux, windows
	Architecture string `json:"architecture"` // amd64, arm64
	OSVersion    string `json:"os_version"`   // e.g., "14.0" for macOS Sonoma
	Kernel       string `json:"kernel"`       // kernel version
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

	// Machine identity
	MachineType MachineType   `json:"machine_type"`
	Platform    *PlatformInfo `json:"platform,omitempty"`
	TimeOfDay   TimeOfDay     `json:"time_of_day"`

	// Optional metrics (nil if unavailable on platform)
	LoadAverages *LoadAverages `json:"load_averages,omitempty"`
	SwapTotal    *uint64       `json:"swap_total,omitempty"`
	SwapUsed     *uint64       `json:"swap_used,omitempty"`
	SwapPercent  *float64      `json:"swap_percent,omitempty"`
	Battery      *BatteryInfo  `json:"battery,omitempty"`
	ProcessCount *int          `json:"process_count,omitempty"`
	NetworkIO    *NetworkIO    `json:"network_io,omitempty"`
	Thermal      *ThermalInfo  `json:"thermal,omitempty"`
	Fans         []*FanInfo    `json:"fans,omitempty"`
	GPU          *GPUInfo      `json:"gpu,omitempty"`
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

	now := time.Now()
	snapshot := &Snapshot{
		Timestamp:     now,
		Uptime:        time.Duration(uptimeSeconds) * time.Second,
		MemoryTotal:   memInfo.Total,
		MemoryUsed:    memInfo.Used,
		MemoryPercent: memInfo.UsedPercent,
		CPUPercent:    cpuPercent,
		DiskTotal:     diskInfo.Total,
		DiskUsed:      diskInfo.Used,
		DiskPercent:   diskInfo.UsedPercent,
		TimeOfDay:     getTimeOfDay(now),
		MachineType:   MachineTypeUnknown, // Will be detected below
	}

	// Collect optional metrics (failures are silently ignored)
	gatherOptionalMetrics(snapshot, memInfo)

	// Detect machine type (depends on optional metrics being gathered first)
	snapshot.MachineType = detectMachineType(snapshot)

	return snapshot, nil
}

// getTimeOfDay returns the general time period based on hour
func getTimeOfDay(t time.Time) TimeOfDay {
	hour := t.Hour()
	switch {
	case hour >= 0 && hour < 6:
		return TimeOfDayNight
	case hour >= 6 && hour < 12:
		return TimeOfDayMorning
	case hour >= 12 && hour < 18:
		return TimeOfDayAfternoon
	default:
		return TimeOfDayEvening
	}
}

// gatherOptionalMetrics collects platform-specific metrics that may not be available
func gatherOptionalMetrics(snapshot *Snapshot, memInfo *mem.VirtualMemoryStat) {
	// Platform info
	gatherPlatformInfo(snapshot)

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

	// Thermal info
	gatherThermalInfo(snapshot)

	// Fan speeds
	gatherFanInfo(snapshot)

	// GPU metrics
	gatherGPUInfo(snapshot)
}

// gatherPlatformInfo collects OS and architecture information
func gatherPlatformInfo(snapshot *Snapshot) {
	info := &PlatformInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	if hostInfo, err := host.Info(); err == nil {
		info.OSVersion = hostInfo.PlatformVersion
		info.Kernel = hostInfo.KernelVersion
	}

	snapshot.Platform = info
}

// gatherThermalInfo collects temperature sensor data
func gatherThermalInfo(snapshot *Snapshot) {
	temps, err := host.SensorsTemperatures()
	if err != nil || len(temps) == 0 {
		return
	}

	thermal := &ThermalInfo{
		SensorCount: len(temps),
	}

	var highestTemp float64
	for _, temp := range temps {
		if temp.Temperature > highestTemp {
			highestTemp = temp.Temperature
		}
		name := temp.SensorKey

		// Look for CPU temperature sensor
		// Common names: "coretemp", "cpu", "CPU", "k10temp", "acpitz"
		if thermal.CPUTemp == nil {
			if contains(name, "cpu") || contains(name, "core") || contains(name, "k10temp") {
				thermal.CPUTemp = &temp.Temperature
			}
		}

		// Look for GPU temperature sensor
		// Common names: "gpu", "radeon", "nvidia", "amdgpu"
		if thermal.GPUTemp == nil {
			if contains(name, "gpu") || contains(name, "radeon") || contains(name, "nvidia") || contains(name, "amdgpu") {
				thermal.GPUTemp = &temp.Temperature
			}
		}
	}

	thermal.HighestTemp = highestTemp
	snapshot.Thermal = thermal
}

// gatherFanInfo collects fan speed data
func gatherFanInfo(snapshot *Snapshot) {
	// gopsutil doesn't have direct fan API, try platform-specific methods
	switch runtime.GOOS {
	case "darwin":
		gatherFanInfoDarwin(snapshot)
	case "linux":
		gatherFanInfoLinux(snapshot)
	}
}

// gatherFanInfoDarwin reads fan info on macOS
func gatherFanInfoDarwin(snapshot *Snapshot) {
	// macOS fan info requires SMC access which needs elevated privileges
	// or third-party tools. Skip for now.
}

// gatherFanInfoLinux reads fan info from sysfs
func gatherFanInfoLinux(snapshot *Snapshot) {
	// Try common hwmon paths for fan speeds
	// /sys/class/hwmon/hwmon*/fan*_input contains RPM values
	hwmonDirs, err := os.ReadDir("/sys/class/hwmon")
	if err != nil {
		return
	}

	for _, hwmon := range hwmonDirs {
		if !hwmon.IsDir() {
			continue
		}
		basePath := "/sys/class/hwmon/" + hwmon.Name()

		// Look for fan inputs
		files, err := os.ReadDir(basePath)
		if err != nil {
			continue
		}

		for _, f := range files {
			name := f.Name()
			if !contains(name, "fan") || !contains(name, "input") {
				continue
			}

			data, err := os.ReadFile(basePath + "/" + name)
			if err != nil {
				continue
			}

			var rpm float64
			if _, err := fmt.Sscanf(string(data), "%f", &rpm); err != nil {
				continue
			}

			if rpm > 0 {
				snapshot.Fans = append(snapshot.Fans, &FanInfo{
					Speed: rpm,
					Name:  name,
				})
			}
		}
	}
}

// gatherGPUInfo collects GPU metrics
func gatherGPUInfo(snapshot *Snapshot) {
	switch runtime.GOOS {
	case "darwin":
		gatherGPUInfoDarwin(snapshot)
	case "linux":
		gatherGPUInfoLinux(snapshot)
	}
}

// gatherGPUInfoDarwin reads GPU info on macOS
func gatherGPUInfoDarwin(snapshot *Snapshot) {
	// macOS GPU metrics require IOKit or Metal APIs
	// Could use `ioreg` but it's complex to parse
	// Skip for now - would need CGO or external tool
}

// gatherGPUInfoLinux reads GPU info from sysfs/nvidia-smi
func gatherGPUInfoLinux(snapshot *Snapshot) {
	// Try nvidia-smi for NVIDIA GPUs
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu,memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err == nil {
		// Parse output: "45, 2048, 8192" (usage%, mem used MB, mem total MB)
		var usage float64
		var memUsed, memTotal uint64
		if _, err := fmt.Sscanf(string(output), "%f, %d, %d", &usage, &memUsed, &memTotal); err == nil {
			memUsedBytes := memUsed * 1024 * 1024
			memTotalBytes := memTotal * 1024 * 1024
			snapshot.GPU = &GPUInfo{
				Usage:       &usage,
				MemoryUsed:  &memUsedBytes,
				MemoryTotal: &memTotalBytes,
			}
			return
		}
	}

	// Try AMD GPU via sysfs
	// /sys/class/drm/card*/device/gpu_busy_percent
	drmDirs, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return
	}

	for _, card := range drmDirs {
		if !contains(card.Name(), "card") || contains(card.Name(), "-") {
			continue
		}
		basePath := "/sys/class/drm/" + card.Name() + "/device"

		// GPU utilization
		if data, err := os.ReadFile(basePath + "/gpu_busy_percent"); err == nil {
			var usage float64
			if _, err := fmt.Sscanf(string(data), "%f", &usage); err == nil {
				snapshot.GPU = &GPUInfo{
					Usage: &usage,
				}

				// Try to get VRAM info
				if usedData, err := os.ReadFile(basePath + "/mem_info_vram_used"); err == nil {
					var used uint64
					if _, err := fmt.Sscanf(string(usedData), "%d", &used); err == nil {
						snapshot.GPU.MemoryUsed = &used
					}
				}
				if totalData, err := os.ReadFile(basePath + "/mem_info_vram_total"); err == nil {
					var total uint64
					if _, err := fmt.Sscanf(string(totalData), "%d", &total); err == nil {
						snapshot.GPU.MemoryTotal = &total
					}
				}
				return
			}
		}
	}
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && (containsLower(s, substr)))
}

func containsLower(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// gatherBatteryInfo attempts to get battery status
func gatherBatteryInfo(snapshot *Snapshot) {
	switch runtime.GOOS {
	case "darwin":
		gatherBatteryDarwin(snapshot)
	case "linux":
		gatherBatteryLinux(snapshot)
	}
}

// gatherBatteryDarwin reads battery info on macOS using pmset
func gatherBatteryDarwin(snapshot *Snapshot) {
	// Use pmset -g batt to get battery info
	// Output format: "Now drawing from 'Battery Power'" or "'AC Power'"
	// "-InternalBattery-0 (id=...)	95%; charging; 0:30 remaining"
	cmd := exec.Command("pmset", "-g", "batt")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := string(output)

	// Check if running on battery or AC
	charging := !containsLower(lines, "battery power")

	// Parse battery percentage
	// Look for pattern like "95%"
	for _, line := range splitLines(lines) {
		if containsLower(line, "internalbattery") {
			// Extract percentage
			pct := extractPercentage(line)
			if pct >= 0 {
				snapshot.Battery = &BatteryInfo{
					Percent:  pct,
					Charging: charging || containsLower(line, "charging"),
				}
				return
			}
		}
	}
}

// gatherBatteryLinux reads battery info from sysfs
func gatherBatteryLinux(snapshot *Snapshot) {
	// Try common battery paths
	paths := []string{
		"/sys/class/power_supply/BAT0",
		"/sys/class/power_supply/BAT1",
	}

	for _, basePath := range paths {
		capacityPath := basePath + "/capacity"
		statusPath := basePath + "/status"

		capacityData, err := os.ReadFile(capacityPath)
		if err != nil {
			continue
		}

		var capacity float64
		if _, err := fmt.Sscanf(string(capacityData), "%f", &capacity); err != nil {
			continue
		}

		charging := false
		if statusData, err := os.ReadFile(statusPath); err == nil {
			status := string(statusData)
			charging = containsLower(status, "charging") && !containsLower(status, "discharging")
		}

		snapshot.Battery = &BatteryInfo{
			Percent:  capacity,
			Charging: charging,
		}
		return
	}
}

// splitLines splits a string into lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// extractPercentage extracts a percentage value from a string like "95%"
func extractPercentage(s string) float64 {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			// Found start of number
			j := i
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
				j++
			}
			// Check if followed by %
			if j < len(s) && s[j] == '%' {
				var pct float64
				if _, err := fmt.Sscanf(s[i:j], "%f", &pct); err == nil {
					return pct
				}
			}
			i = j
		}
	}
	return -1
}

// detectMachineType determines the type of machine based on available info
func detectMachineType(snapshot *Snapshot) MachineType {
	// Check for container first (most specific)
	if isContainer() {
		return MachineTypeContainer
	}

	// Check for VM
	if isVirtualMachine() {
		return MachineTypeVM
	}

	// Check for laptop (has battery)
	if snapshot.Battery != nil {
		return MachineTypeLaptop
	}

	// Check for server indicators
	if isLikelyServer(snapshot) {
		return MachineTypeServer
	}

	// Default to desktop if we have a display-capable OS
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return MachineTypeDesktop
	}

	// Linux without battery could be desktop or server
	return MachineTypeUnknown
}

// isContainer checks if running inside a container
func isContainer() bool {
	// Check for Docker
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup for docker/kubepods
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if containsLower(content, "docker") || containsLower(content, "kubepods") ||
			containsLower(content, "containerd") {
			return true
		}
	}

	return false
}

// isVirtualMachine checks if running inside a VM
func isVirtualMachine() bool {
	// Use gopsutil's virtualization detection
	system, role, err := host.Virtualization()
	if err != nil {
		return false
	}

	// If we're a guest, we're in a VM
	if role == "guest" {
		return true
	}

	// Check for known hypervisors
	knownVMs := []string{"vmware", "virtualbox", "kvm", "qemu", "xen", "hyperv", "parallels"}
	for _, vm := range knownVMs {
		if containsLower(system, vm) {
			return true
		}
	}

	return false
}

// isLikelyServer checks for indicators that this is a server
func isLikelyServer(snapshot *Snapshot) bool {
	// High uptime (> 30 days) suggests server
	if snapshot.Uptime > 30*24*time.Hour {
		return true
	}

	// High process count might indicate server workload
	if snapshot.ProcessCount != nil && *snapshot.ProcessCount > 500 {
		return true
	}

	// No display server on Linux often means server
	if runtime.GOOS == "linux" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return true
		}
	}

	return false
}
