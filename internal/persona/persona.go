package persona

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/cldixon/jernel/internal/config"
)

//go:embed examples/*.md
var examplesFS embed.FS

// Persona defines a character voice for journal entries
type Persona struct {
	Name        string `yaml:"name"`
	Description string
}

// Dir returns the personas directory path
func Dir() (string, error) {
	cfgDir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "personas"), nil
}

// Load reads a persona from a markdown file with frontmatter
func Load(path string) (*Persona, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open persona file: %w", err)
	}
	defer file.Close()

	var p Persona
	content, err := frontmatter.Parse(file, &p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse persona frontmatter: %w", err)
	}

	p.Description = strings.TrimSpace(string(content))

	return &p, nil
}

// LoadByName looks for a persona file in the personas directory
func LoadByName(name string) (*Persona, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(dir, name+".md")
	return Load(path)
}

// Save writes a persona to disk as markdown with frontmatter
func Save(p *Persona) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create personas directory: %w", err)
	}

	content := fmt.Sprintf(`---
name: %s
---

%s
`, p.Name, strings.TrimSpace(p.Description))

	path := filepath.Join(dir, p.Name+".md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write persona: %w", err)
	}

	return nil
}

// List returns all available persona names
func List() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			name := entry.Name()[:len(entry.Name())-3] // strip .md
			names = append(names, name)
		}
	}

	return names, nil
}

// Get retrieves a persona by name
func Get(name string) (*Persona, error) {
	p, err := LoadByName(name)
	if err != nil {
		return nil, fmt.Errorf("persona '%s' not found (check ~/.config/jernel/personas/)", name)
	}
	return p, nil
}

// Create creates a new persona file with a template and returns the file path
func Create(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create personas directory: %w", err)
	}

	path := filepath.Join(dir, name+".md")

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("persona '%s' already exists", name)
	}

	// Create template content
	content := fmt.Sprintf(`---
name: %s
---

Describe this persona's writing style, tone, and perspective here.

Consider including:
- The persona's voice and personality
- Their typical mood or emotional range
- How they might perceive system metrics
- Any quirks or unique characteristics
`, name)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create persona file: %w", err)
	}

	return path, nil
}

// Delete removes a persona file
func Delete(name string) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, name+".md")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("persona '%s' not found", name)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete persona: %w", err)
	}

	return nil
}

// ListExamples returns the names of all bundled example personas
func ListExamples() ([]string, error) {
	entries, err := examplesFS.ReadDir("examples")
	if err != nil {
		return nil, fmt.Errorf("failed to read examples: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			name := entry.Name()[:len(entry.Name())-3] // strip .md
			names = append(names, name)
		}
	}

	return names, nil
}

// GetExample loads a bundled example persona by name
func GetExample(name string) (*Persona, error) {
	data, err := examplesFS.ReadFile("examples/" + name + ".md")
	if err != nil {
		return nil, fmt.Errorf("example persona '%s' not found", name)
	}

	var p Persona
	content, err := frontmatter.Parse(bytes.NewReader(data), &p)
	if err != nil {
		return nil, fmt.Errorf("failed to parse example persona: %w", err)
	}

	p.Description = strings.TrimSpace(string(content))
	return &p, nil
}
