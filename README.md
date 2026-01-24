# jernel

A journal for your machine's soul. jernel gives your computer a voice by translating system metrics into personal journal entries using LLMs.

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
- `system_prompt.md` — customizable LLM instructions
- `personas/` — character definitions for journal entries

Set your Anthropic API key:
```bash
export ANTHROPIC_API_KEY=your-key-here
```

## Usage

Create a new journal entry:
```bash
jernel new
```

Use a specific persona:
```bash
jernel new --persona dramatic
```

List recent entries:
```bash
jernel read --list
```

Read a specific entry:
```bash
jernel read 1
```

Read the most recent entry:
```bash
jernel read
```

## Personas

Personas are markdown files in `~/.config/jernel/personas/`. Create your own:
```markdown
---
name: anxious
---

A nervous computer who worries about everything. High CPU usage triggers panic,
low disk space causes existential dread. Always anticipating the next crash.
```

Then use it:
```bash
jernel new --persona anxious
```

## License

MIT
