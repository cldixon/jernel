package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/system_prompt.md
var DefaultSystemPrompt string

//go:embed defaults/personas/*.md
var defaultPersonasFS embed.FS

// DaemonConfig holds settings for autonomous entry generation
type DaemonConfig struct {
	Rate       int      `yaml:"rate"`        // number of entries per period
	RatePeriod string   `yaml:"rate_period"` // "hour", "day", or "week"
	Personas   []string `yaml:"personas"`    // personas to randomly select from
}

// Config holds application-level settings
type Config struct {
	Provider       string        `yaml:"provider"`
	Model          string        `yaml:"model"`
	DefaultPersona string        `yaml:"default_persona"`
	Daemon         *DaemonConfig `yaml:"daemon,omitempty"`
}

// DefaultDaemonConfig returns sensible defaults for daemon settings
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		Rate:       3,
		RatePeriod: "day",
		Personas:   []string{},
	}
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Provider:       "anthropic",
		Model:          "claude-4-5-sonnet-20250514",
		DefaultPersona: "default",
		Daemon:         DefaultDaemonConfig(),
	}
}

// Dir returns the jernel config directory path
func Dir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "jernel"), nil
}

// Path returns the full path to config.yaml
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads the config file, returning defaults if it doesn't exist
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure daemon config has defaults if not specified
	if cfg.Daemon == nil {
		cfg.Daemon = DefaultDaemonConfig()
	}

	return cfg, nil
}

// Save writes the config to disk
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := Path()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Init ensures the config directory exists with all necessary files
func Init() error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	// Create directory structure
	personaDir := filepath.Join(dir, "personas")
	if err := os.MkdirAll(personaDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directories: %w", err)
	}

	// Write default config if it doesn't exist
	cfgPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := Save(DefaultConfig()); err != nil {
			return fmt.Errorf("failed to write default config: %w", err)
		}
	}

	// Write system prompt if it doesn't exist
	promptPath := filepath.Join(dir, "system_prompt.md")
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		if err := os.WriteFile(promptPath, []byte(DefaultSystemPrompt), 0644); err != nil {
			return fmt.Errorf("failed to write system prompt: %w", err)
		}
	}

	// Write default personas if personas directory is empty
	entries, _ := os.ReadDir(personaDir)
	if len(entries) == 0 {
		personaFiles, err := defaultPersonasFS.ReadDir("defaults/personas")
		if err != nil {
			return fmt.Errorf("failed to read embedded personas: %w", err)
		}

		for _, file := range personaFiles {
			content, err := defaultPersonasFS.ReadFile("defaults/personas/" + file.Name())
			if err != nil {
				return fmt.Errorf("failed to read embedded persona %s: %w", file.Name(), err)
			}

			destPath := filepath.Join(personaDir, file.Name())
			if err := os.WriteFile(destPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write persona %s: %w", file.Name(), err)
			}
		}
	}

	return nil
}

// SystemPromptPath returns the path to the system prompt file
func SystemPromptPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "system_prompt.md"), nil
}

// LoadSystemPrompt reads the system prompt from disk
func LoadSystemPrompt() (string, error) {
	path, err := SystemPromptPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSystemPrompt, nil
		}
		return "", fmt.Errorf("failed to read system prompt: %w", err)
	}

	return string(data), nil
}
