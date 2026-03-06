package backup

import (
	"os"
	"testing"
)

type testData struct {
	Groups   map[string][]string `json:"groups"`
	Patterns map[string][]string `json:"patterns,omitempty"`
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	data := testData{
		Groups: map[string][]string{
			"default": {"/path/to/a.md", "/path/to/b.md"},
			"docs":    {"/path/to/c.md"},
		},
		Patterns: map[string][]string{
			"default": {"/path/to/*.md"},
		},
	}

	if err := Save(6275, data); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if !Exists(6275) {
		t.Fatal("Exists returned false after Save")
	}

	var loaded testData
	if err := Load(6275, &loaded); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(loaded.Groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(loaded.Groups))
	}
	if len(loaded.Groups["default"]) != 2 {
		t.Fatalf("got %d files in default group, want 2", len(loaded.Groups["default"]))
	}
	if loaded.Groups["default"][0] != "/path/to/a.md" {
		t.Fatalf("got %s, want /path/to/a.md", loaded.Groups["default"][0])
	}
	if len(loaded.Patterns["default"]) != 1 {
		t.Fatalf("got %d patterns, want 1", len(loaded.Patterns["default"]))
	}
}

func TestLoadNonExistent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var loaded testData
	if err := Load(9999, &loaded); err != nil {
		t.Fatalf("Load should return nil error for non-existent file, got: %v", err)
	}
	if len(loaded.Groups) != 0 {
		t.Fatalf("loaded data should be empty, got %v", loaded)
	}
}

func TestRemove(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	data := testData{
		Groups: map[string][]string{
			"default": {"/path/to/a.md"},
		},
	}
	if err := Save(6275, data); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if err := Remove(6275); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	if Exists(6275) {
		t.Fatal("Exists returned true after Remove")
	}
}

func TestRemoveNonExistent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := Remove(9999); err != nil {
		t.Fatalf("Remove should return nil error for non-existent file, got: %v", err)
	}
}

func TestSaveOverwrite(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	data1 := testData{
		Groups: map[string][]string{
			"default": {"/path/to/a.md"},
		},
	}
	if err := Save(6275, data1); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data2 := testData{
		Groups: map[string][]string{
			"default": {"/path/to/b.md", "/path/to/c.md"},
		},
	}
	if err := Save(6275, data2); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	var loaded testData
	if err := Load(6275, &loaded); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded.Groups["default"]) != 2 {
		t.Fatalf("got %d files, want 2", len(loaded.Groups["default"]))
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	p, err := Path(6275)
	if err != nil {
		t.Fatalf("Path returned error: %v", err)
	}

	want := dir + "/mo/backup/mo-6275.json"
	if p != want {
		t.Fatalf("got %s, want %s", p, want)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	// Directory does not exist yet
	backupDir := dir + "/mo/backup"
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatal("backup directory should not exist before Save")
	}

	data := testData{Groups: map[string][]string{"default": {"/a.md"}}}
	if err := Save(6275, data); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("backup directory should exist after Save: %v", err)
	}
}
