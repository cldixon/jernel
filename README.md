# jernel

A journal for your machine's soul. jernel gives your computer a voice by translating system metrics into personal journal entries using LLMs.

<!-- Screenshot placeholder: Add a screenshot of the TUI here -->
<!-- ![jernel TUI](assets/screenshot.png) -->

## Installation

Quick install (macOS arm64, Linux amd64):
```bash
curl -sSL https://raw.githubusercontent.com/cldixon/jernel/main/install.sh | sh
```

Or download a binary directly from [Releases](https://github.com/cldixon/jernel/releases).

Or install with Go:
```bash
go install github.com/cldixon/jernel@latest
```

Or build from source:
```bash
git clone https://github.com/cldixon/jernel.git
cd jernel
go build -o jernel .
```

Check your installed version:
```bash
jernel --version
```

## Configuration

On first run, jernel creates a config directory at `~/.config/jernel/` with:

- `config.yaml` — model settings and defaults
- `system_prompt.md` — system prompt for the LLM
- `message_prompt.md` — customizable entry generation template
- `personas/` — character definitions for journal entries

Set your Anthropic API key:
```bash
export ANTHROPIC_API_KEY=your-key-here
```

## TUI Quick Start

The easiest way to use jernel is through the interactive TUI:

```bash
jernel open
```

This opens a full-screen terminal interface where you can:
- Browse and read journal entries
- Generate new entries with persona selection
- Create and edit personas with the built-in editor
- Start/stop the daemon for automatic entry generation
- View settings and configuration paths

![](assets/jernel_tui_demo.png)

## CLI Commands

### Entries

```bash
# Create a new entry (uses default persona)
jernel entry create

# Create with a specific persona
jernel entry create --persona dramatic

# List recent entries
jernel entry list

# List entries for a specific persona
jernel entry list --persona dramatic

# Read the most recent entry
jernel entry read

# Read a specific entry by ID
jernel entry read 5
```

### Personas

```bash
# List all personas
jernel persona list

# Create a new persona (opens template file)
jernel persona create my_persona

# Delete a persona (with option to delete associated entries)
jernel persona delete my_persona
```

### Daemon

The daemon runs in the background and generates entries automatically at random intervals.

```bash
# Start the daemon (default: 3 entries per day)
jernel daemon start

# Start with custom rate
jernel daemon start --rate 5 --rate-period day

# Start with specific personas (randomly selected for each entry)
jernel daemon start --personas "poor_charlie,prof_whitlock"

# Check daemon status
jernel daemon status

# Stop the daemon
jernel daemon stop
```

### Other Commands

```bash
# Open the interactive TUI
jernel open

# Delete all entries (with confirmation)
jernel reset
```

## Personas

Personas define the voice and personality for journal entries. They are markdown files stored in `~/.config/jernel/personas/`.

Create a new persona:
```bash
jernel persona create prof_whitlock
```

This creates a template file you can edit. Example persona:

```markdown
---
name: prof_whitlock
---

Professor Whitlock is a retired professor of english literature. His expertise was in Shakesepeare, and when under duress, is known for slipping into the rhythm and vocaburaly of the Bard himself. Somehow his days after professional life are more stressful than those before, and he seems to have developed remarkably bad luck in the twilight of his life. This has also led to a general crankiness, and an odd combination of everything being too loud while also struggling to hear others speak. His writings as of late tend toward a cantankerous recap of his daily struggles.

```

Use it when creating entries:
```bash
jernel entry create --persona prof_whitlock
```

Or select it in the TUI when pressing `n` to create a new entry.

## Context Continuity

jernel includes your most recent entries (default: 3) when generating new ones, allowing the LLM to maintain narrative continuity and build on previous themes. Configure this in `config.yaml`:

```yaml
context_entries: 3  # number of previous entries to include
```

Set to `0` to disable context continuity.

## Customization

### Message Prompt

The `~/.config/jernel/message_prompt.md` file controls how entries are generated. It's a Go template with access to:

- `{{.Persona}}` — the persona description
- `{{.MachineType}}` — laptop, desktop, server, etc.
- `{{.TimeOfDay}}` — morning, afternoon, evening, night
- `{{.CPUPercent}}`, `{{.MemoryPercent}}`, etc. — system metrics
- `{{.PreviousEntries}}` — recent entries for context

Power users can customize this template to change the entry format or add additional instructions.

### System Prompt

The `~/.config/jernel/system_prompt.md` file contains the system-level instructions for the LLM. Edit this to change the fundamental behavior of entry generation.

## Development

### Running Tests

```bash
go test ./...
```

With verbose output:
```bash
go test ./... -v
```

With race detection:
```bash
go test ./... -race
```

### Building

```bash
go build -o jernel .
```

## License

MIT
