package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStateWithGCS(t *testing.T, cacheDir string) *State {
	t.Helper()
	s := newTestState(t)
	// Set up a minimal GCSManager (no real client — we only test handleGCSNotification,
	// which doesn't call the GCS client directly except for InvalidateCache).
	s.gcs = &GCSManager{
		cacheDir: cacheDir,
		buckets: map[string]BucketState{
			"my-bucket": {Subscription: "test-sub"},
		},
	}
	return s
}

func TestHandleGCSNotification_Finalize_NewFile(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	// Register a GCS pattern.
	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{Name: "reports"}

	// Simulate OBJECT_FINALIZE for a new file.
	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":  "reports/2026/jan.md",
		"eventType": "OBJECT_FINALIZE",
	})

	// File should be added to the reports group.
	g := s.groups["reports"]
	if len(g.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(g.Files))
	}
	if g.Files[0].Path != "gs://my-bucket/reports/2026/jan.md" {
		t.Errorf("unexpected path: %s", g.Files[0].Path)
	}
	if g.Files[0].Name != "jan.md" {
		t.Errorf("unexpected name: %s", g.Files[0].Name)
	}
}

func TestHandleGCSNotification_Finalize_ExistingFile(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	uri := "gs://my-bucket/reports/file.md"
	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{
		Name: "reports",
		Files: []*FileEntry{{
			Name: "file.md",
			ID:   FileID(uri),
			Path: uri,
		}},
	}

	// Write a cached file so InvalidateCache has something to remove.
	cachePath := filepath.Join(cacheDir, "my-bucket", "reports", "file.md")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":  "reports/file.md",
		"eventType": "OBJECT_FINALIZE",
	})

	// File count should remain 1 (not duplicated).
	if len(s.groups["reports"].Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(s.groups["reports"].Files))
	}

	// Cache should have been invalidated.
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("expected cache file to be removed")
	}
}

func TestHandleGCSNotification_Delete(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	uri := "gs://my-bucket/reports/file.md"
	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{
		Name: "reports",
		Files: []*FileEntry{{
			Name: "file.md",
			ID:   FileID(uri),
			Path: uri,
		}},
	}

	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":  "reports/file.md",
		"eventType": "OBJECT_DELETE",
	})

	if len(s.groups["reports"].Files) != 0 {
		t.Fatalf("expected 0 files after delete, got %d", len(s.groups["reports"].Files))
	}
}

func TestHandleGCSNotification_Archive_Overwrite(t *testing.T) {
	// On versioned buckets, overwriting a file emits OBJECT_ARCHIVE for the old version
	// with overwrittenByGeneration set. This should be ignored — OBJECT_FINALIZE handles it.
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	uri := "gs://my-bucket/reports/file.md"
	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{
		Name: "reports",
		Files: []*FileEntry{{
			Name: "file.md",
			ID:   FileID(uri),
			Path: uri,
		}},
	}

	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":                "reports/file.md",
		"eventType":              "OBJECT_ARCHIVE",
		"objectGeneration":       "1000",
		"overwrittenByGeneration": "2000",
	})

	// File should still exist — this was an overwrite, not a deletion.
	if len(s.groups["reports"].Files) != 1 {
		t.Fatalf("expected 1 file after overwrite archive, got %d", len(s.groups["reports"].Files))
	}
}

func TestHandleGCSNotification_Archive_Deletion(t *testing.T) {
	// On versioned buckets, deleting a file emits OBJECT_ARCHIVE without
	// overwrittenByGeneration. This should remove the file.
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	uri := "gs://my-bucket/reports/file.md"
	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{
		Name: "reports",
		Files: []*FileEntry{{
			Name: "file.md",
			ID:   FileID(uri),
			Path: uri,
		}},
	}

	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":          "reports/file.md",
		"eventType":         "OBJECT_ARCHIVE",
		"objectGeneration":  "2000",
	})

	if len(s.groups["reports"].Files) != 0 {
		t.Fatalf("expected 0 files after deletion archive, got %d", len(s.groups["reports"].Files))
	}
}

func TestHandleGCSNotification_NoMatch(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	s.patterns = append(s.patterns, &GlobPattern{
		Pattern:      "gs://my-bucket/reports/**/*.md",
		PatternSlash: "gs://my-bucket/reports/**/*.md",
		Group:        "reports",
	})
	s.groups["reports"] = &Group{Name: "reports"}

	// Object doesn't match the pattern.
	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":  "other/file.txt",
		"eventType": "OBJECT_FINALIZE",
	})

	if len(s.groups["reports"].Files) != 0 {
		t.Fatalf("expected 0 files for non-matching object, got %d", len(s.groups["reports"].Files))
	}
}

func TestHandleGCSNotification_MissingAttributes(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	s.groups["reports"] = &Group{Name: "reports"}

	// Missing objectId — should be silently ignored.
	s.handleGCSNotification("my-bucket", map[string]string{
		"eventType": "OBJECT_FINALIZE",
	})

	// Missing eventType — should be silently ignored.
	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId": "reports/file.md",
	})

	if len(s.groups["reports"].Files) != 0 {
		t.Fatalf("expected 0 files for messages with missing attributes, got %d", len(s.groups["reports"].Files))
	}
}

func TestHandleGCSNotification_MultipleGroups(t *testing.T) {
	cacheDir := t.TempDir()
	s := newTestStateWithGCS(t, cacheDir)

	// Two patterns in different groups that both match the same prefix.
	s.patterns = append(s.patterns,
		&GlobPattern{
			Pattern:      "gs://my-bucket/shared/**/*.md",
			PatternSlash: "gs://my-bucket/shared/**/*.md",
			Group:        "group-a",
		},
		&GlobPattern{
			Pattern:      "gs://my-bucket/shared/**/*.md",
			PatternSlash: "gs://my-bucket/shared/**/*.md",
			Group:        "group-b",
		},
	)
	s.groups["group-a"] = &Group{Name: "group-a"}
	s.groups["group-b"] = &Group{Name: "group-b"}

	s.handleGCSNotification("my-bucket", map[string]string{
		"objectId":  "shared/doc.md",
		"eventType": "OBJECT_FINALIZE",
	})

	if len(s.groups["group-a"].Files) != 1 {
		t.Errorf("group-a: expected 1 file, got %d", len(s.groups["group-a"].Files))
	}
	if len(s.groups["group-b"].Files) != 1 {
		t.Errorf("group-b: expected 1 file, got %d", len(s.groups["group-b"].Files))
	}
}

func TestFindFileByPath(t *testing.T) {
	s := newTestState(t)
	s.groups["default"] = &Group{
		Name: "default",
		Files: []*FileEntry{
			{Name: "a.md", ID: "aaaa", Path: "/local/a.md"},
			{Name: "b.md", ID: "bbbb", Path: "gs://bucket/b.md"},
		},
	}

	if f := s.FindFileByPath("/local/a.md"); f == nil || f.ID != "aaaa" {
		t.Error("expected to find local file")
	}
	if f := s.FindFileByPath("gs://bucket/b.md"); f == nil || f.ID != "bbbb" {
		t.Error("expected to find GCS file")
	}
	if f := s.FindFileByPath("gs://bucket/missing.md"); f != nil {
		t.Error("expected nil for missing file")
	}
}

func TestRemoveFileByPath(t *testing.T) {
	s := newTestState(t)
	s.groups["default"] = &Group{
		Name: "default",
		Files: []*FileEntry{
			{Name: "a.md", ID: "aaaa", Path: "gs://bucket/a.md"},
			{Name: "b.md", ID: "bbbb", Path: "gs://bucket/b.md"},
		},
	}

	s.RemoveFileByPath("gs://bucket/a.md")

	if len(s.groups["default"].Files) != 1 {
		t.Fatalf("expected 1 file after remove, got %d", len(s.groups["default"].Files))
	}
	if s.groups["default"].Files[0].Path != "gs://bucket/b.md" {
		t.Errorf("wrong file remaining: %s", s.groups["default"].Files[0].Path)
	}
}

func TestRemoveFileByPath_MultipleGroups(t *testing.T) {
	s := newTestState(t)
	uri := "gs://bucket/shared.md"
	s.groups["group-a"] = &Group{
		Name:  "group-a",
		Files: []*FileEntry{{Name: "shared.md", ID: FileID(uri), Path: uri}},
	}
	s.groups["group-b"] = &Group{
		Name:  "group-b",
		Files: []*FileEntry{{Name: "shared.md", ID: FileID(uri), Path: uri}},
	}

	s.RemoveFileByPath(uri)

	if len(s.groups["group-a"].Files) != 0 {
		t.Errorf("group-a: expected 0 files, got %d", len(s.groups["group-a"].Files))
	}
	if len(s.groups["group-b"].Files) != 0 {
		t.Errorf("group-b: expected 0 files, got %d", len(s.groups["group-b"].Files))
	}
}

func TestGCSGroupError_ExposedInGroups(t *testing.T) {
	s := newTestState(t)
	s.groups["reports"] = &Group{Name: "reports"}

	retryAt := time.Now().Add(30 * time.Second)
	s.gcsErrors["reports"] = gcsGroupError{
		Message: "failed to list gs://bucket: permission denied",
		RetryAt: retryAt,
	}

	// Simulate what handleGroups does: overlay error state.
	groups := s.Groups()
	s.mu.RLock()
	for i := range groups {
		if e, ok := s.gcsErrors[groups[i].Name]; ok {
			groups[i].Error = e.Message
			groups[i].RetryAt = e.RetryAt.UTC().Format(time.RFC3339)
		}
	}
	s.mu.RUnlock()

	var found bool
	for _, g := range groups {
		if g.Name == "reports" {
			found = true
			if g.Error == "" {
				t.Error("expected error to be set on reports group")
			}
			if g.RetryAt == "" {
				t.Error("expected retryAt to be set on reports group")
			}
		}
	}
	if !found {
		t.Error("reports group not found")
	}
}
