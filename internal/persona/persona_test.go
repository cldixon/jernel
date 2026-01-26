package persona

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestEnv creates a temporary home directory for testing
func setupTestEnv(t *testing.T) (string, func()) {
	t.Helper()

	tmpHome, err := os.MkdirTemp("", "jernel-persona-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	// Create personas directory
	personaDir := filepath.Join(tmpHome, ".config", "jernel", "personas")
	if err := os.MkdirAll(personaDir, 0755); err != nil {
		t.Fatalf("failed to create persona dir: %v", err)
	}

	cleanup := func() {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tmpHome)
	}

	return personaDir, cleanup
}

// TestPersonaLoadAndParseFrontmatter verifies that persona files with
// frontmatter are correctly parsed.
func TestPersonaLoadAndParseFrontmatter(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a test persona file
	content := `---
name: test_persona
---

This is the persona description.
It can span multiple lines.

And have multiple paragraphs.
`
	path := filepath.Join(personaDir, "test_persona.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test persona: %v", err)
	}

	// Load and verify
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if p.Name != "test_persona" {
		t.Errorf("expected name 'test_persona', got %q", p.Name)
	}

	if !strings.Contains(p.Description, "persona description") {
		t.Errorf("description missing expected content: %q", p.Description)
	}

	if !strings.Contains(p.Description, "multiple paragraphs") {
		t.Errorf("description should preserve multiple paragraphs: %q", p.Description)
	}
}

// TestPersonaLoadByName verifies that personas can be loaded by name
// from the personas directory.
func TestPersonaLoadByName(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	content := `---
name: my_persona
---

A creative writing persona.
`
	if err := os.WriteFile(filepath.Join(personaDir, "my_persona.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write persona: %v", err)
	}

	p, err := LoadByName("my_persona")
	if err != nil {
		t.Fatalf("LoadByName() failed: %v", err)
	}

	if p.Name != "my_persona" {
		t.Errorf("name mismatch: %q", p.Name)
	}
}

// TestPersonaList verifies that List() returns all persona names.
func TestPersonaList(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create multiple personas
	personas := []string{"alice", "bob", "charlie"}
	for _, name := range personas {
		content := "---\nname: " + name + "\n---\nDescription for " + name
		path := filepath.Join(personaDir, name+".md")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write persona %s: %v", name, err)
		}
	}

	// Also create a non-md file that should be ignored
	if err := os.WriteFile(filepath.Join(personaDir, "notes.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}

	// List personas
	names, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(names) != 3 {
		t.Errorf("expected 3 personas, got %d: %v", len(names), names)
	}

	// Verify all expected names are present
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	for _, expected := range personas {
		if !nameSet[expected] {
			t.Errorf("missing expected persona: %s", expected)
		}
	}
}

// TestPersonaListEmptyDirectory verifies List() handles empty directories.
func TestPersonaListEmptyDirectory(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	names, err := List()
	if err != nil {
		t.Fatalf("List() failed on empty dir: %v", err)
	}

	if len(names) != 0 {
		t.Errorf("expected empty list, got %v", names)
	}
}

// TestPersonaCreate verifies that Create() creates a proper template file.
func TestPersonaCreate(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	path, err := Create("new_persona")
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	expectedPath := filepath.Join(personaDir, "new_persona.md")
	if path != expectedPath {
		t.Errorf("path mismatch: expected %q, got %q", expectedPath, path)
	}

	// Verify file exists and has valid content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read created file: %v", err)
	}

	if !strings.Contains(string(content), "name: new_persona") {
		t.Error("created file missing frontmatter name")
	}

	if !strings.Contains(string(content), "writing style") {
		t.Error("created file missing template instructions")
	}

	// Verify it's loadable
	p, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load created persona: %v", err)
	}

	if p.Name != "new_persona" {
		t.Errorf("loaded name mismatch: %q", p.Name)
	}
}

// TestPersonaCreateAlreadyExists verifies Create() fails for existing personas.
func TestPersonaCreateAlreadyExists(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create an existing persona
	content := "---\nname: existing\n---\nAlready here"
	if err := os.WriteFile(filepath.Join(personaDir, "existing.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write existing persona: %v", err)
	}

	// Try to create again
	_, err := Create("existing")
	if err == nil {
		t.Error("expected error when creating existing persona, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %v", err)
	}
}

// TestPersonaDelete verifies that Delete() removes persona files.
func TestPersonaDelete(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a persona to delete
	path := filepath.Join(personaDir, "deleteme.md")
	if err := os.WriteFile(path, []byte("---\nname: deleteme\n---\nTemp"), 0644); err != nil {
		t.Fatalf("failed to write persona: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("persona file should exist before delete")
	}

	// Delete it
	if err := Delete("deleteme"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("persona file should not exist after delete")
	}
}

// TestPersonaDeleteNotFound verifies Delete() returns error for missing personas.
func TestPersonaDeleteNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	err := Delete("nonexistent")
	if err == nil {
		t.Error("expected error when deleting nonexistent persona")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

// TestPersonaSave verifies that Save() writes personas correctly.
func TestPersonaSave(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	p := &Persona{
		Name:        "saved_persona",
		Description: "A saved persona description.\n\nWith multiple paragraphs.",
	}

	if err := Save(p); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(personaDir, "saved_persona.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("saved persona file should exist")
	}

	// Load it back and verify
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load saved persona: %v", err)
	}

	if loaded.Name != p.Name {
		t.Errorf("name mismatch: expected %q, got %q", p.Name, loaded.Name)
	}

	if !strings.Contains(loaded.Description, "multiple paragraphs") {
		t.Errorf("description mismatch: %q", loaded.Description)
	}
}

// TestPersonaGetNotFound verifies Get() returns a helpful error message.
func TestPersonaGetNotFound(t *testing.T) {
	_, cleanup := setupTestEnv(t)
	defer cleanup()

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent persona")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}

	if !strings.Contains(err.Error(), "~/.config/jernel/personas") {
		t.Errorf("error should hint at personas directory: %v", err)
	}
}

// TestPersonaMalformedFrontmatter verifies Load() handles bad frontmatter.
func TestPersonaMalformedFrontmatter(t *testing.T) {
	personaDir, cleanup := setupTestEnv(t)
	defer cleanup()

	// Create a file with invalid frontmatter
	content := `---
name: [invalid yaml
---

Description
`
	path := filepath.Join(personaDir, "bad.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write bad persona: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for malformed frontmatter")
	}

	if !strings.Contains(err.Error(), "frontmatter") {
		t.Errorf("error should mention frontmatter: %v", err)
	}
}

// TestExamplePersonasExist verifies bundled example personas are loadable.
func TestExamplePersonasExist(t *testing.T) {
	examples, err := ListExamples()
	if err != nil {
		t.Fatalf("ListExamples() failed: %v", err)
	}

	if len(examples) == 0 {
		t.Error("expected at least one bundled example persona")
	}

	// Try to load each example
	for _, name := range examples {
		p, err := GetExample(name)
		if err != nil {
			t.Errorf("failed to load example %q: %v", name, err)
			continue
		}

		if p.Name == "" {
			t.Errorf("example %q has empty name", name)
		}

		if len(p.Description) < 10 {
			t.Errorf("example %q has very short description: %q", name, p.Description)
		}
	}
}
