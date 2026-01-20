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
}

// NewContext creates a prompt context from a persona description and metrics snapshot
func NewContext(personaDescription string, snapshot *metrics.Snapshot) *Context {
	return &Context{
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

## Instructions
Write a short, first-person journal entry (2-3 paragraphs) reflecting on how you feel right now.
Be introspective and express emotions based on your current physical state.
Write as if this is your private diary—be honest and vulnerable.`

// Render executes a template string with the given context
func Render(tmpl string, ctx *Context) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
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
