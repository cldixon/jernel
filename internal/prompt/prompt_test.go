package prompt

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cldixon/jernel/internal/config"
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

	ctx := NewContext("Test persona description", snapshot, nil)
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

	ctx := NewContext("Minimal persona", snapshot, nil)
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

// TestRenderMessagePromptWithPreviousEntries verifies that previous entries
// are correctly rendered in the message prompt template.
func TestRenderMessagePromptWithPreviousEntries(t *testing.T) {
	// Setup temp home with config
	tmpHome, err := os.MkdirTemp("", "jernel-prompt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Initialize config to create message_prompt.md
	if err := config.Init(); err != nil {
		t.Fatalf("config.Init() failed: %v", err)
	}

	snapshot := &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        2 * time.Hour,
		MemoryTotal:   16 * 1024 * 1024 * 1024,
		MemoryUsed:    8 * 1024 * 1024 * 1024,
		MemoryPercent: 50.0,
		CPUPercent:    30.0,
		DiskTotal:     500 * 1024 * 1024 * 1024,
		DiskUsed:      200 * 1024 * 1024 * 1024,
		DiskPercent:   40.0,
		MachineType:   metrics.MachineTypeLaptop,
		TimeOfDay:     metrics.TimeOfDayMorning,
		Platform: &metrics.PlatformInfo{
			OS:           "darwin",
			OSVersion:    "14.0",
			Architecture: "arm64",
		},
	}

	previousEntries := []PreviousEntry{
		{
			Date:    "Monday, January 20, 2025 at 10:00 AM",
			Content: "I woke up feeling refreshed today. The CPU load was light.",
		},
		{
			Date:    "Sunday, January 19, 2025 at 8:00 PM",
			Content: "Evening thoughts: memory pressure has been building.",
		},
	}

	ctx := NewContext("A thoughtful AI persona", snapshot, previousEntries)
	rendered, err := RenderMessagePrompt(ctx)
	if err != nil {
		t.Fatalf("RenderMessagePrompt failed: %v", err)
	}

	// Verify persona is included
	if !strings.Contains(rendered, "A thoughtful AI persona") {
		t.Error("Expected persona description in output")
	}

	// Verify machine context
	if !strings.Contains(rendered, "laptop") {
		t.Error("Expected machine type in output")
	}
	if !strings.Contains(rendered, "morning") {
		t.Error("Expected time of day in output")
	}
	if !strings.Contains(rendered, "macOS") {
		t.Error("Expected platform in output")
	}

	// Verify metrics
	if !strings.Contains(rendered, "30.0%") {
		t.Error("Expected CPU percentage in output")
	}

	// Verify previous entries section appears
	if !strings.Contains(rendered, "Previous Entries") {
		t.Error("Expected 'Previous Entries' section header")
	}
	if !strings.Contains(rendered, "Monday, January 20, 2025") {
		t.Error("Expected first previous entry date")
	}
	if !strings.Contains(rendered, "I woke up feeling refreshed") {
		t.Error("Expected first previous entry content")
	}
	if !strings.Contains(rendered, "Evening thoughts") {
		t.Error("Expected second previous entry content")
	}
}

// TestRenderMessagePromptWithoutPreviousEntries verifies that the previous
// entries section is omitted when there are no previous entries.
func TestRenderMessagePromptWithoutPreviousEntries(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "jernel-prompt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	if err := config.Init(); err != nil {
		t.Fatalf("config.Init() failed: %v", err)
	}

	snapshot := &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        1 * time.Hour,
		MemoryTotal:   16 * 1024 * 1024 * 1024,
		MemoryUsed:    4 * 1024 * 1024 * 1024,
		MemoryPercent: 25.0,
		CPUPercent:    15.0,
		DiskTotal:     500 * 1024 * 1024 * 1024,
		DiskUsed:      100 * 1024 * 1024 * 1024,
		DiskPercent:   20.0,
		MachineType:   metrics.MachineTypeDesktop,
		TimeOfDay:     metrics.TimeOfDayNight,
	}

	// No previous entries
	ctx := NewContext("Fresh persona", snapshot, nil)
	rendered, err := RenderMessagePrompt(ctx)
	if err != nil {
		t.Fatalf("RenderMessagePrompt failed: %v", err)
	}

	// Verify persona and metrics are present
	if !strings.Contains(rendered, "Fresh persona") {
		t.Error("Expected persona in output")
	}
	if !strings.Contains(rendered, "15.0%") {
		t.Error("Expected CPU percentage in output")
	}

	// Verify previous entries section is NOT present
	if strings.Contains(rendered, "Previous Entries") {
		t.Error("Previous Entries section should not appear when empty")
	}
}

// TestNewContextMapsAllMetricFields verifies that NewContext correctly
// maps all fields from a metrics.Snapshot to the Context struct.
func TestNewContextMapsAllMetricFields(t *testing.T) {
	cpuTemp := 65.0
	gpuTemp := 70.0
	gpuUsage := 45.0
	load1, load5, load15 := 1.0, 1.5, 2.0
	swapTotal, swapUsed := uint64(4*1024*1024*1024), uint64(1*1024*1024*1024)
	swapPct := 25.0
	procCount := 300

	snapshot := &metrics.Snapshot{
		Timestamp:     time.Date(2025, 1, 15, 14, 30, 0, 0, time.UTC),
		Uptime:        48 * time.Hour,
		MemoryTotal:   32 * 1024 * 1024 * 1024,
		MemoryUsed:    16 * 1024 * 1024 * 1024,
		MemoryPercent: 50.0,
		CPUPercent:    35.5,
		DiskTotal:     1024 * 1024 * 1024 * 1024,
		DiskUsed:      512 * 1024 * 1024 * 1024,
		DiskPercent:   50.0,
		MachineType:   metrics.MachineTypeServer,
		TimeOfDay:     metrics.TimeOfDayAfternoon,
		Platform: &metrics.PlatformInfo{
			OS:           "linux",
			OSVersion:    "5.15",
			Architecture: "amd64",
		},
		LoadAverages: &metrics.LoadAverages{
			Load1:  load1,
			Load5:  load5,
			Load15: load15,
		},
		SwapTotal:    &swapTotal,
		SwapUsed:     &swapUsed,
		SwapPercent:  &swapPct,
		ProcessCount: &procCount,
		NetworkIO: &metrics.NetworkIO{
			BytesSent: 100 * 1024 * 1024 * 1024,
			BytesRecv: 200 * 1024 * 1024 * 1024,
		},
		Battery: &metrics.BatteryInfo{
			Percent:  80.0,
			Charging: false,
		},
		Thermal: &metrics.ThermalInfo{
			CPUTemp: &cpuTemp,
			GPUTemp: &gpuTemp,
		},
		GPU: &metrics.GPUInfo{
			Usage: &gpuUsage,
		},
		Fans: []*metrics.FanInfo{
			{Name: "fan1", Speed: 1500},
			{Name: "fan2", Speed: 1600},
		},
	}

	previousEntries := []PreviousEntry{
		{Date: "Yesterday", Content: "Test entry"},
	}

	ctx := NewContext("Test persona", snapshot, previousEntries)

	// Core fields
	if ctx.Persona != "Test persona" {
		t.Errorf("Persona mismatch: %q", ctx.Persona)
	}
	if ctx.Uptime != "48h0m0s" {
		t.Errorf("Uptime mismatch: %q", ctx.Uptime)
	}
	if ctx.CPUPercent != 35.5 {
		t.Errorf("CPUPercent mismatch: %.1f", ctx.CPUPercent)
	}

	// Machine identity
	if ctx.MachineType != "server" {
		t.Errorf("MachineType mismatch: %q", ctx.MachineType)
	}
	if ctx.TimeOfDay != "afternoon" {
		t.Errorf("TimeOfDay mismatch: %q", ctx.TimeOfDay)
	}
	if !strings.Contains(ctx.Platform, "Linux") {
		t.Errorf("Platform should contain 'Linux': %q", ctx.Platform)
	}

	// Optional metrics
	if !ctx.HasLoadAverage() {
		t.Error("HasLoadAverage should be true")
	}
	if !ctx.HasSwap() {
		t.Error("HasSwap should be true")
	}
	if !ctx.HasProcessCount() {
		t.Error("HasProcessCount should be true")
	}
	if !ctx.HasNetwork() {
		t.Error("HasNetwork should be true")
	}
	if !ctx.HasBattery() {
		t.Error("HasBattery should be true")
	}
	if !ctx.HasCPUTemp() {
		t.Error("HasCPUTemp should be true")
	}
	if !ctx.HasGPUTemp() {
		t.Error("HasGPUTemp should be true")
	}
	if !ctx.HasGPUUsage() {
		t.Error("HasGPUUsage should be true")
	}
	if !ctx.HasFanSpeed() {
		t.Error("HasFanSpeed should be true")
	}

	// Verify fan speed is averaged
	expectedFanAvg := 1550.0 // (1500 + 1600) / 2
	if *ctx.FanSpeed != expectedFanAvg {
		t.Errorf("FanSpeed should be averaged: expected %.0f, got %.0f", expectedFanAvg, *ctx.FanSpeed)
	}

	// Previous entries
	if !ctx.HasPreviousEntries() {
		t.Error("HasPreviousEntries should be true")
	}
	if len(ctx.PreviousEntries) != 1 {
		t.Errorf("Expected 1 previous entry, got %d", len(ctx.PreviousEntries))
	}
}

// TestHasPreviousEntriesWithEmptySlice verifies that HasPreviousEntries
// returns false for empty slice (not just nil).
func TestHasPreviousEntriesWithEmptySlice(t *testing.T) {
	snapshot := &metrics.Snapshot{
		Timestamp:     time.Now(),
		Uptime:        1 * time.Hour,
		MemoryTotal:   16 * 1024 * 1024 * 1024,
		MemoryUsed:    8 * 1024 * 1024 * 1024,
		MemoryPercent: 50.0,
		CPUPercent:    25.0,
		DiskTotal:     500 * 1024 * 1024 * 1024,
		DiskUsed:      250 * 1024 * 1024 * 1024,
		DiskPercent:   50.0,
	}

	// Empty slice (not nil)
	ctx := NewContext("Test", snapshot, []PreviousEntry{})
	if ctx.HasPreviousEntries() {
		t.Error("HasPreviousEntries should return false for empty slice")
	}
}
