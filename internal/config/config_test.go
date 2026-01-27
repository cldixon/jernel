package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInitCreatesRequiredFiles verifies that Init() creates all necessary
// configuration files and directories on a fresh setup.
func TestInitCreatesRequiredFiles(t *testing.T) {
	// Create a temporary directory to act as home
	tmpHome, err := os.MkdirTemp("", "jernel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	// Override HOME for the test
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Run Init
	if err := Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify config directory was created
	configDir := filepath.Join(tmpHome, ".config", "jernel")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("config directory was not created")
	}

	// Verify personas directory was created
	personaDir := filepath.Join(configDir, "personas")
	if _, err := os.Stat(personaDir); os.IsNotExist(err) {
		t.Error("personas directory was not created")
	}

	// Verify config.yaml was created
	configPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config.yaml was not created")
	}

	// Verify system_prompt.md was created
	systemPromptPath := filepath.Join(configDir, "system_prompt.md")
	if _, err := os.Stat(systemPromptPath); os.IsNotExist(err) {
		t.Error("system_prompt.md was not created")
	}

	// Verify message_prompt.md was created
	messagePromptPath := filepath.Join(configDir, "message_prompt.md")
	if _, err := os.Stat(messagePromptPath); os.IsNotExist(err) {
		t.Error("message_prompt.md was not created")
	}

	// Verify config.yaml has valid content
	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load config after Init: %v", err)
	}
	if cfg.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.Provider)
	}
	if cfg.ContextEntries != 3 {
		t.Errorf("expected context_entries 3, got %d", cfg.ContextEntries)
	}

	// Verify system_prompt.md has content
	systemPrompt, err := LoadSystemPrompt()
	if err != nil {
		t.Fatalf("failed to load system prompt: %v", err)
	}
	if len(systemPrompt) == 0 {
		t.Error("system_prompt.md is empty")
	}

	// Verify message_prompt.md has content and contains expected placeholders
	messagePrompt, err := LoadMessagePrompt()
	if err != nil {
		t.Fatalf("failed to load message prompt: %v", err)
	}
	if len(messagePrompt) == 0 {
		t.Error("message_prompt.md is empty")
	}
	// Check for key template placeholders
	expectedPlaceholders := []string{
		"{{.Persona}}",
		"{{.MachineType}}",
		".CPUPercent",
		".HasPreviousEntries",
	}
	for _, placeholder := range expectedPlaceholders {
		if !contains(messagePrompt, placeholder) {
			t.Errorf("message_prompt.md missing placeholder: %s", placeholder)
		}
	}
}

// TestInitIsIdempotent verifies that running Init() multiple times
// doesn't overwrite existing config files.
func TestInitIsIdempotent(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "jernel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// First Init
	if err := Init(); err != nil {
		t.Fatalf("first Init() failed: %v", err)
	}

	// Modify config.yaml
	cfg, _ := Load()
	cfg.DefaultPersona = "custom_persona"
	if err := Save(cfg); err != nil {
		t.Fatalf("failed to save modified config: %v", err)
	}

	// Second Init should not overwrite
	if err := Init(); err != nil {
		t.Fatalf("second Init() failed: %v", err)
	}

	// Verify config still has our modification
	cfg2, _ := Load()
	if cfg2.DefaultPersona != "custom_persona" {
		t.Errorf("Init() overwrote config.yaml: expected 'custom_persona', got %q", cfg2.DefaultPersona)
	}
}

// TestLoadReturnsDefaultsForMissingFile verifies that Load() returns
// sensible defaults when config.yaml doesn't exist.
func TestLoadReturnsDefaultsForMissingFile(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "jernel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Don't run Init - just try to load
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed on missing config: %v", err)
	}

	// Should have defaults
	if cfg.Provider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got %q", cfg.Provider)
	}
	if cfg.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("unexpected default model: %q", cfg.Model)
	}
	if cfg.Daemon == nil {
		t.Error("expected default daemon config, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
