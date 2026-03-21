package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFile(t *testing.T) {
	t.Run("parses all fields", func(t *testing.T) {
		f := writeTempConfig(t, `
port: 8080
bind: 0.0.0.0
foreground: true
no-open: true
no-restart: true
no-delete: true
read-only: true
shareable: true
server: true
shouty: true
groups:
  - name: docs
    watch:
      - /docs/**/*.md
  - name: default
    files:
      - /README.md
`)
		cfg, err := loadConfigFile(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != 8080 {
			t.Errorf("Port = %d, want 8080", cfg.Port)
		}
		if cfg.Bind != "0.0.0.0" {
			t.Errorf("Bind = %q, want 0.0.0.0", cfg.Bind)
		}
		if !cfg.Foreground {
			t.Error("Foreground should be true")
		}
		if !cfg.NoOpen {
			t.Error("NoOpen should be true")
		}
		if !cfg.NoRestart {
			t.Error("NoRestart should be true")
		}
		if !cfg.NoDelete {
			t.Error("NoDelete should be true")
		}
		if !cfg.ReadOnly {
			t.Error("ReadOnly should be true")
		}
		if !cfg.Shareable {
			t.Error("Shareable should be true")
		}
		if !cfg.ServerMode {
			t.Error("ServerMode should be true")
		}
		if !cfg.Shouty {
			t.Error("Shouty should be true")
		}
		if len(cfg.Groups) != 2 {
			t.Fatalf("len(Groups) = %d, want 2", len(cfg.Groups))
		}
		if cfg.Groups[0].Name != "docs" {
			t.Errorf("Groups[0].Name = %q, want docs", cfg.Groups[0].Name)
		}
		if len(cfg.Groups[0].Watch) != 1 || cfg.Groups[0].Watch[0] != "/docs/**/*.md" {
			t.Errorf("Groups[0].Watch = %v, want [/docs/**/*.md]", cfg.Groups[0].Watch)
		}
		if len(cfg.Groups[1].Files) != 1 || cfg.Groups[1].Files[0] != "/README.md" {
			t.Errorf("Groups[1].Files = %v, want [/README.md]", cfg.Groups[1].Files)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := loadConfigFile("/nonexistent/path/mo.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		f := writeTempConfig(t, "port: [not a number")
		_, err := loadConfigFile(f)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("empty file returns zero-value config", func(t *testing.T) {
		f := writeTempConfig(t, "")
		cfg, err := loadConfigFile(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Port != 0 || cfg.Bind != "" || cfg.Foreground || len(cfg.Groups) != 0 {
			t.Errorf("empty config should produce zero values, got %+v", cfg)
		}
	})
}

func TestApplyConfig(t *testing.T) {
	t.Run("config values applied when CLI flag not set", func(t *testing.T) {
		// Save and restore all vars touched by applyConfig
		origPort, origBind, origForeground := port, bind, foreground
		origNoOpen, origNoRestart, origNoDelete := noOpen, noRestart, noDelete
		origReadOnly, origShareable, origShouty := readOnly, shareable, shouty
		origServerMode := serverMode
		defer func() {
			port, bind, foreground = origPort, origBind, origForeground
			noOpen, noRestart, noDelete = origNoOpen, origNoRestart, origNoDelete
			readOnly, shareable, shouty = origReadOnly, origShareable, origShouty
			serverMode = origServerMode
		}()

		cfg := &configFile{
			Port:       9000,
			Bind:       "0.0.0.0",
			Foreground: true,
			NoOpen:     true,
			NoRestart:  true,
			NoDelete:   true,
			ReadOnly:   true,
			Shareable:  true,
			ServerMode: true,
			Shouty:     true,
		}

		applyConfig(rootCmd, cfg)

		if port != 9000 {
			t.Errorf("port = %d, want 9000", port)
		}
		if bind != "0.0.0.0" {
			t.Errorf("bind = %q, want 0.0.0.0", bind)
		}
		if !foreground {
			t.Error("foreground should be true")
		}
		if !noOpen {
			t.Error("noOpen should be true")
		}
		if !noRestart {
			t.Error("noRestart should be true")
		}
		if !noDelete {
			t.Error("noDelete should be true")
		}
		if !readOnly {
			t.Error("readOnly should be true")
		}
		if !shareable {
			t.Error("shareable should be true")
		}
		if !serverMode {
			t.Error("serverMode should be true")
		}
		if !shouty {
			t.Error("shouty should be true")
		}
	})

	t.Run("CLI flag takes precedence over config", func(t *testing.T) {
		origPort := port
		defer func() {
			port = origPort
			rootCmd.Flags().Lookup("port").Changed = false
		}()

		// Simulate --port 7777 on CLI
		if err := rootCmd.Flags().Set("port", "7777"); err != nil {
			t.Fatalf("Set port: %v", err)
		}

		cfg := &configFile{Port: 9000}
		applyConfig(rootCmd, cfg)

		if port != 7777 {
			t.Errorf("port = %d, want 7777 (CLI should win over config)", port)
		}
	})

	t.Run("zero port in config does not override CLI default", func(t *testing.T) {
		origPort := port
		defer func() { port = origPort }()

		port = 6275 // default
		cfg := &configFile{Port: 0}
		applyConfig(rootCmd, cfg)

		if port != 6275 {
			t.Errorf("port = %d, want 6275 (zero port in config should not override)", port)
		}
	})
}

func TestBuildGroupsFromConfig(t *testing.T) {
	t.Run("watch patterns are resolved to absolute paths", func(t *testing.T) {
		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "docs", Watch: []string{"**/*.md"}},
			},
		}
		filesByGroup, patternsByGroup, err := buildGroupsFromConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(filesByGroup) != 0 {
			t.Errorf("expected no files, got %v", filesByGroup)
		}
		if len(patternsByGroup["docs"]) != 1 {
			t.Fatalf("expected 1 pattern for docs, got %v", patternsByGroup["docs"])
		}
		if !filepath.IsAbs(patternsByGroup["docs"][0]) {
			t.Errorf("pattern %q should be absolute", patternsByGroup["docs"][0])
		}
	})

	t.Run("files are resolved to absolute paths", func(t *testing.T) {
		dir := t.TempDir()
		f := filepath.Join(dir, "README.md")
		os.WriteFile(f, []byte("# hi"), 0o600) //nolint:errcheck

		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "default", Files: []string{f}},
			},
		}
		filesByGroup, _, err := buildGroupsFromConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(filesByGroup["default"]) != 1 {
			t.Fatalf("expected 1 file for default, got %v", filesByGroup["default"])
		}
		if !filepath.IsAbs(filesByGroup["default"][0]) {
			t.Errorf("file %q should be absolute", filesByGroup["default"][0])
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "default", Files: []string{"/nonexistent/file.md"}},
			},
		}
		_, _, err := buildGroupsFromConfig(cfg)
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("pattern without glob chars returns error", func(t *testing.T) {
		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "docs", Watch: []string{"/docs/README.md"}},
			},
		}
		_, _, err := buildGroupsFromConfig(cfg)
		if err == nil {
			t.Fatal("expected error for pattern without glob chars")
		}
	})

	t.Run("invalid group name returns error", func(t *testing.T) {
		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "my?group"}, // ? is not allowed in group names
			},
		}
		_, _, err := buildGroupsFromConfig(cfg)
		if err == nil {
			t.Fatal("expected error for invalid group name")
		}
	})

	t.Run("multiple groups are all processed", func(t *testing.T) {
		cfg := &configFile{
			Groups: []groupConfig{
				{Name: "api", Watch: []string{"api/**/*.md"}},
				{Name: "guides", Watch: []string{"guides/**/*.md"}},
			},
		}
		_, patternsByGroup, err := buildGroupsFromConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(patternsByGroup["api"]) != 1 {
			t.Errorf("expected 1 pattern for api, got %v", patternsByGroup["api"])
		}
		if len(patternsByGroup["guides"]) != 1 {
			t.Errorf("expected 1 pattern for guides, got %v", patternsByGroup["guides"])
		}
	})

	t.Run("empty groups produces empty maps", func(t *testing.T) {
		cfg := &configFile{}
		filesByGroup, patternsByGroup, err := buildGroupsFromConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(filesByGroup) != 0 {
			t.Errorf("expected empty filesByGroup, got %v", filesByGroup)
		}
		if len(patternsByGroup) != 0 {
			t.Errorf("expected empty patternsByGroup, got %v", patternsByGroup)
		}
	})
}

// writeTempConfig writes content to a temp file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "mo-config-*.yaml")
	if err != nil {
		t.Fatalf("creating temp config: %v", err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return f.Name()
}
