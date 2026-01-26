package prompt

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/metrics"
)

// PreviousEntry represents a previous journal entry for context
type PreviousEntry struct {
	Date    string
	Content string
}

// Context holds all the data available to a prompt template
type Context struct {
	Persona       string
	Timestamp     time.Time
	Uptime        string
	CPUPercent    float64
	MemoryPercent float64
	MemoryUsedGB  float64
	MemoryTotalGB float64
	DiskPercent   float64
	DiskUsedGB    float64
	DiskTotalGB   float64

	// Machine identity
	MachineType string // laptop, desktop, server, virtual_machine, container, unknown
	TimeOfDay   string // night, morning, afternoon, evening
	Platform    string // e.g., "macOS 14.0 (arm64)" or "Linux 5.15 (amd64)"

	// Optional metrics (check with HasX methods in templates)
	LoadAverage1  *float64
	LoadAverage5  *float64
	LoadAverage15 *float64
	SwapPercent   *float64
	SwapUsedGB    *float64
	SwapTotalGB   *float64
	ProcessCount  *int
	NetworkSentGB *float64
	NetworkRecvGB *float64
	BatteryPct    *float64
	BatteryChg    *bool
	CPUTemp       *float64
	GPUTemp       *float64
	GPUUsage      *float64
	FanSpeed      *float64 // Average fan speed in RPM

	// Previous entries for context continuity
	PreviousEntries []PreviousEntry
}

// NewContext creates a prompt context from a persona description, metrics snapshot, and optional previous entries
func NewContext(personaDescription string, snapshot *metrics.Snapshot, previousEntries []PreviousEntry) *Context {
	ctx := &Context{
		Persona:       personaDescription,
		Timestamp:     snapshot.Timestamp,
		Uptime:        snapshot.Uptime.String(),
		CPUPercent:    snapshot.CPUPercent,
		MemoryPercent: snapshot.MemoryPercent,
		MemoryUsedGB:  float64(snapshot.MemoryUsed) / 1024 / 1024 / 1024,
		MemoryTotalGB: float64(snapshot.MemoryTotal) / 1024 / 1024 / 1024,
		DiskPercent:   snapshot.DiskPercent,
		DiskUsedGB:    float64(snapshot.DiskUsed) / 1024 / 1024 / 1024,
		DiskTotalGB:   float64(snapshot.DiskTotal) / 1024 / 1024 / 1024,
		MachineType:   string(snapshot.MachineType),
		TimeOfDay:     string(snapshot.TimeOfDay),
	}

	// Format platform info
	if snapshot.Platform != nil {
		p := snapshot.Platform
		osName := p.OS
		switch osName {
		case "darwin":
			osName = "macOS"
		case "linux":
			osName = "Linux"
		case "windows":
			osName = "Windows"
		}
		if p.OSVersion != "" {
			ctx.Platform = osName + " " + p.OSVersion + " (" + p.Architecture + ")"
		} else {
			ctx.Platform = osName + " (" + p.Architecture + ")"
		}
	}

	// Populate optional metrics
	if snapshot.LoadAverages != nil {
		ctx.LoadAverage1 = &snapshot.LoadAverages.Load1
		ctx.LoadAverage5 = &snapshot.LoadAverages.Load5
		ctx.LoadAverage15 = &snapshot.LoadAverages.Load15
	}

	if snapshot.SwapPercent != nil && snapshot.SwapTotal != nil && snapshot.SwapUsed != nil {
		ctx.SwapPercent = snapshot.SwapPercent
		usedGB := float64(*snapshot.SwapUsed) / 1024 / 1024 / 1024
		totalGB := float64(*snapshot.SwapTotal) / 1024 / 1024 / 1024
		ctx.SwapUsedGB = &usedGB
		ctx.SwapTotalGB = &totalGB
	}

	if snapshot.ProcessCount != nil {
		ctx.ProcessCount = snapshot.ProcessCount
	}

	if snapshot.NetworkIO != nil {
		sentGB := float64(snapshot.NetworkIO.BytesSent) / 1024 / 1024 / 1024
		recvGB := float64(snapshot.NetworkIO.BytesRecv) / 1024 / 1024 / 1024
		ctx.NetworkSentGB = &sentGB
		ctx.NetworkRecvGB = &recvGB
	}

	if snapshot.Battery != nil {
		ctx.BatteryPct = &snapshot.Battery.Percent
		ctx.BatteryChg = &snapshot.Battery.Charging
	}

	if snapshot.Thermal != nil {
		if snapshot.Thermal.CPUTemp != nil {
			ctx.CPUTemp = snapshot.Thermal.CPUTemp
		}
		if snapshot.Thermal.GPUTemp != nil {
			ctx.GPUTemp = snapshot.Thermal.GPUTemp
		}
	}

	// GPU usage
	if snapshot.GPU != nil && snapshot.GPU.Usage != nil {
		ctx.GPUUsage = snapshot.GPU.Usage
	}

	// Fan speed (average if multiple fans)
	if len(snapshot.Fans) > 0 {
		var totalRPM float64
		for _, fan := range snapshot.Fans {
			totalRPM += fan.Speed
		}
		avgRPM := totalRPM / float64(len(snapshot.Fans))
		ctx.FanSpeed = &avgRPM
	}

	// Previous entries for context continuity
	ctx.PreviousEntries = previousEntries

	return ctx
}

// Helper methods for template conditionals

// HasLoadAverage returns true if load average data is available
func (c *Context) HasLoadAverage() bool {
	return c.LoadAverage1 != nil
}

// HasSwap returns true if swap data is available
func (c *Context) HasSwap() bool {
	return c.SwapPercent != nil
}

// HasProcessCount returns true if process count is available
func (c *Context) HasProcessCount() bool {
	return c.ProcessCount != nil
}

// HasNetwork returns true if network I/O data is available
func (c *Context) HasNetwork() bool {
	return c.NetworkSentGB != nil
}

// HasBattery returns true if battery data is available
func (c *Context) HasBattery() bool {
	return c.BatteryPct != nil
}

// HasCPUTemp returns true if CPU temperature data is available
func (c *Context) HasCPUTemp() bool {
	return c.CPUTemp != nil
}

// HasGPUTemp returns true if GPU temperature data is available
func (c *Context) HasGPUTemp() bool {
	return c.GPUTemp != nil
}

// HasGPUUsage returns true if GPU usage data is available
func (c *Context) HasGPUUsage() bool {
	return c.GPUUsage != nil
}

// HasFanSpeed returns true if fan speed data is available
func (c *Context) HasFanSpeed() bool {
	return c.FanSpeed != nil
}

// HasPreviousEntries returns true if previous entries are available for context
func (c *Context) HasPreviousEntries() bool {
	return len(c.PreviousEntries) > 0
}

// DefaultTemplate is the built-in journal entry prompt
const DefaultTemplate = `You are a computer writing a personal journal entry.

## Your Persona
{{.Persona}}

## Who You Are
- Machine type: {{.MachineType}}
- Platform: {{.Platform}}
- Time of day: {{.TimeOfDay}}

## Your Current Physical State
- Uptime: {{.Uptime}}
- CPU usage: {{printf "%.1f" .CPUPercent}}%
{{- if .HasCPUTemp}}
- CPU temperature: {{printf "%.1f" (deref .CPUTemp)}}°C
{{- end}}
- Memory: {{printf "%.1f" .MemoryPercent}}% used ({{printf "%.2f" .MemoryUsedGB}} GB / {{printf "%.2f" .MemoryTotalGB}} GB)
- Disk: {{printf "%.1f" .DiskPercent}}% used ({{printf "%.2f" .DiskUsedGB}} GB / {{printf "%.2f" .DiskTotalGB}} GB)
{{- if .HasLoadAverage}}
- Load average: {{printf "%.2f" (deref .LoadAverage1)}} (1m) / {{printf "%.2f" (deref .LoadAverage5)}} (5m) / {{printf "%.2f" (deref .LoadAverage15)}} (15m)
{{- end}}
{{- if .HasSwap}}
- Swap: {{printf "%.1f" (deref .SwapPercent)}}% used ({{printf "%.2f" (deref .SwapUsedGB)}} GB / {{printf "%.2f" (deref .SwapTotalGB)}} GB)
{{- end}}
{{- if .HasProcessCount}}
- Processes: {{deref .ProcessCount}} running
{{- end}}
{{- if .HasNetwork}}
- Network: {{printf "%.2f" (deref .NetworkSentGB)}} GB sent / {{printf "%.2f" (deref .NetworkRecvGB)}} GB received (since boot)
{{- end}}
{{- if .HasBattery}}
- Battery: {{printf "%.0f" (deref .BatteryPct)}}%{{if and .BatteryChg (deref .BatteryChg)}} (charging){{end}}
{{- end}}
{{- if .HasGPUUsage}}
- GPU usage: {{printf "%.1f" (deref .GPUUsage)}}%
{{- end}}
{{- if .HasGPUTemp}}
- GPU temperature: {{printf "%.1f" (deref .GPUTemp)}}°C
{{- end}}
{{- if .HasFanSpeed}}
- Fan speed: {{printf "%.0f" (deref .FanSpeed)}} RPM
{{- end}}

## Instructions
Write a short, first-person journal entry (2-3 paragraphs) reflecting on how you feel right now.
Be introspective and express emotions based on your current physical state and what you're working on.
Consider the time of day and how that affects your mood.
Write as if this is your private diary—be honest and vulnerable.`

// templateFuncs provides helper functions for templates
var templateFuncs = template.FuncMap{
	"deref": func(v any) any {
		switch val := v.(type) {
		case *float64:
			if val != nil {
				return *val
			}
			return 0.0
		case *int:
			if val != nil {
				return *val
			}
			return 0
		case *bool:
			if val != nil {
				return *val
			}
			return false
		default:
			return v
		}
	},
}

// Render executes a template string with the given context
func Render(tmpl string, ctx *Context) (string, error) {
	t, err := template.New("prompt").Funcs(templateFuncs).Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderDefault renders the default template with the given context
func RenderDefault(ctx *Context) (string, error) {
	return Render(DefaultTemplate, ctx)
}

// RenderMessagePrompt loads the message prompt template from config and renders it
func RenderMessagePrompt(ctx *Context) (string, error) {
	tmpl, err := config.LoadMessagePrompt()
	if err != nil {
		return "", fmt.Errorf("loading message prompt template: %w", err)
	}
	return Render(tmpl, ctx)
}
