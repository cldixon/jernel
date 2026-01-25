package prompt

import (
	"strings"
	"testing"
	"time"

	"github.com/cldixon/jernel/internal/metrics"
)

func TestRenderDefaultWithOptionalMetrics(t *testing.T) {
	// Create a snapshot with all optional metrics populated
	load1, load5, load15 := 1.5, 2.0, 1.8
	swapTotal, swapUsed := uint64(8*1024*1024*1024), uint64(1*1024*1024*1024)
	swapPercent := 12.5
	processCount := 250
	batteryPct := 75.0
	batteryChg := true

	snapshot := &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        24 * time.Hour,
		MemoryTotal:   16 * 1024 * 1024 * 1024,
		MemoryUsed:    8 * 1024 * 1024 * 1024,
		MemoryPercent: 50.0,
		CPUPercent:    25.0,
		DiskTotal:     500 * 1024 * 1024 * 1024,
		DiskUsed:      250 * 1024 * 1024 * 1024,
		DiskPercent:   50.0,
		LoadAverages: &metrics.LoadAverages{
			Load1:  load1,
			Load5:  load5,
			Load15: load15,
		},
		SwapTotal:    &swapTotal,
		SwapUsed:     &swapUsed,
		SwapPercent:  &swapPercent,
		ProcessCount: &processCount,
		NetworkIO: &metrics.NetworkIO{
			BytesSent: 5 * 1024 * 1024 * 1024,
			BytesRecv: 10 * 1024 * 1024 * 1024,
		},
		Battery: &metrics.BatteryInfo{
			Percent:  batteryPct,
			Charging: batteryChg,
		},
	}

	ctx := NewContext("Test persona description", snapshot)
	rendered, err := RenderDefault(ctx)
	if err != nil {
		t.Fatalf("RenderDefault failed: %v", err)
	}

	// Verify required fields are present
	if !strings.Contains(rendered, "CPU usage: 25.0%") {
		t.Error("Expected CPU usage in output")
	}
	if !strings.Contains(rendered, "Memory: 50.0%") {
		t.Error("Expected memory usage in output")
	}

	// Verify optional fields are present
	if !strings.Contains(rendered, "Load average:") {
		t.Error("Expected load average in output")
	}
	if !strings.Contains(rendered, "Swap:") {
		t.Error("Expected swap in output")
	}
	if !strings.Contains(rendered, "Processes: 250") {
		t.Error("Expected process count in output")
	}
	if !strings.Contains(rendered, "Network:") {
		t.Error("Expected network in output")
	}
	if !strings.Contains(rendered, "Battery: 75%") {
		t.Error("Expected battery in output")
	}
	if !strings.Contains(rendered, "(charging)") {
		t.Error("Expected charging indicator in output")
	}
}

func TestRenderDefaultWithoutOptionalMetrics(t *testing.T) {
	// Create a snapshot with only required metrics
	snapshot := &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        1 * time.Hour,
		MemoryTotal:   16 * 1024 * 1024 * 1024,
		MemoryUsed:    4 * 1024 * 1024 * 1024,
		MemoryPercent: 25.0,
		CPUPercent:    10.0,
		DiskTotal:     500 * 1024 * 1024 * 1024,
		DiskUsed:      100 * 1024 * 1024 * 1024,
		DiskPercent:   20.0,
		// All optional fields are nil
	}

	ctx := NewContext("Minimal persona", snapshot)
	rendered, err := RenderDefault(ctx)
	if err != nil {
		t.Fatalf("RenderDefault failed: %v", err)
	}

	// Verify required fields are present
	if !strings.Contains(rendered, "CPU usage: 10.0%") {
		t.Error("Expected CPU usage in output")
	}

	// Verify optional fields are NOT present
	if strings.Contains(rendered, "Load average:") {
		t.Error("Load average should not be in output when nil")
	}
	if strings.Contains(rendered, "Swap:") {
		t.Error("Swap should not be in output when nil")
	}
	if strings.Contains(rendered, "Processes:") {
		t.Error("Process count should not be in output when nil")
	}
	if strings.Contains(rendered, "Network:") {
		t.Error("Network should not be in output when nil")
	}
	if strings.Contains(rendered, "Battery:") {
		t.Error("Battery should not be in output when nil")
	}
}
