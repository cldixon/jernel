package prompt

import (
	"bytes"
	"text/template"
	"time"

	"github.com/cldixon/jernel/internal/metrics"
)

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
}

// NewContext creates a prompt context from a persona description and metrics snapshot
func NewContext(personaDescription string, snapshot *metrics.Snapshot) *Context {
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

// DefaultTemplate is the built-in journal entry prompt
const DefaultTemplate = `You are a computer writing a personal journal entry.

## Your Persona
{{.Persona}}

## Your Current Physical State
- Uptime: {{.Uptime}}
- CPU usage: {{printf "%.1f" .CPUPercent}}%
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

## Instructions
Write a short, first-person journal entry (2-3 paragraphs) reflecting on how you feel right now.
Be introspective and express emotions based on your current physical state.
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
