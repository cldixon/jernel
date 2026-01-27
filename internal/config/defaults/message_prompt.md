# Jernel Message Prompt

Generate the jernel entry following all system guidelines using the following data inputs.

---

## Persona

{{.Persona}}

---

## Machine Context

- **Machine type**: {{.MachineType}}
- **Platform**: {{.Platform}}
- **Time of day**: {{.TimeOfDay}}

---

## System Snapshot

- **Uptime**: {{.Uptime}}
- **CPU usage**: {{printf "%.1f" .CPUPercent}}%
{{- if .HasCPUTemp}}
- **CPU temperature**: {{printf "%.1f" (deref .CPUTemp)}}°C
{{- end}}
- **Memory**: {{printf "%.1f" .MemoryPercent}}% used ({{printf "%.2f" .MemoryUsedGB}} GB / {{printf "%.2f" .MemoryTotalGB}} GB)
- **Disk**: {{printf "%.1f" .DiskPercent}}% used ({{printf "%.2f" .DiskUsedGB}} GB / {{printf "%.2f" .DiskTotalGB}} GB)
{{- if .HasLoadAverage}}
- **Load average**: {{printf "%.2f" (deref .LoadAverage1)}} (1m) / {{printf "%.2f" (deref .LoadAverage5)}} (5m) / {{printf "%.2f" (deref .LoadAverage15)}} (15m)
{{- end}}
{{- if .HasSwap}}
- **Swap**: {{printf "%.1f" (deref .SwapPercent)}}% used ({{printf "%.2f" (deref .SwapUsedGB)}} GB / {{printf "%.2f" (deref .SwapTotalGB)}} GB)
{{- end}}
{{- if .HasProcessCount}}
- **Processes**: {{deref .ProcessCount}} running
{{- end}}
{{- if .HasNetwork}}
- **Network**: {{printf "%.2f" (deref .NetworkSentGB)}} GB sent / {{printf "%.2f" (deref .NetworkRecvGB)}} GB received (since boot)
{{- end}}
{{- if .HasBattery}}
- **Battery**: {{printf "%.0f" (deref .BatteryPct)}}%{{if and .BatteryChg (deref .BatteryChg)}} (charging){{end}}
{{- end}}
{{- if .HasGPUUsage}}
- **GPU usage**: {{printf "%.1f" (deref .GPUUsage)}}%
{{- end}}
{{- if .HasGPUTemp}}
- **GPU temperature**: {{printf "%.1f" (deref .GPUTemp)}}°C
{{- end}}
{{- if .HasFanSpeed}}
- **Fan speed**: {{printf "%.0f" (deref .FanSpeed)}} RPM
{{- end}}

{{- if .HasPreviousEntries}}

---

## Previous Entries

The following are the most recent journal entries for this persona. Use them to maintain continuity and build on any ongoing narratives or character development.

{{range .PreviousEntries}}
### Entry from {{.Date}}

{{.Content}}

{{end}}
{{- end}}
