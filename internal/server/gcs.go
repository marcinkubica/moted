package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/bmatcuk/doublestar/v4"
	"google.golang.org/api/iterator"
)

// IsGCSPath returns true if the path starts with "gs://".
func IsGCSPath(path string) bool {
	return strings.HasPrefix(path, "gs://")
}

// ParseGCSURI parses "gs://bucket/path/to/object" into bucket and object.
func ParseGCSURI(uri string) (bucket, object string, err error) {
	if !strings.HasPrefix(uri, "gs://") {
		return "", "", fmt.Errorf("not a GCS URI: %s", uri)
	}
	rest := strings.TrimPrefix(uri, "gs://")
	bucket, object, found := strings.Cut(rest, "/")
	if !found {
		return bucket, "", nil
	}
	return bucket, object, nil
}

// GCSManager handles GCS operations for all configured buckets.
type GCSManager struct {
	client   *storage.Client
	project  string
	cacheDir string
	buckets  map[string]BucketState
}

// BucketState holds per-bucket configuration.
type BucketState struct {
	Subscription string
}

// NewGCSManager creates a GCS manager using Application Default Credentials.
func NewGCSManager(ctx context.Context, project, cacheDir string, buckets map[string]string) (*GCSManager, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	bs := make(map[string]BucketState, len(buckets))
	for name, sub := range buckets {
		bs[name] = BucketState{Subscription: sub}
	}
	return &GCSManager{
		client:   client,
		project:  project,
		cacheDir: cacheDir,
		buckets:  bs,
	}, nil
}

// Read downloads a GCS object. Returns cached content if available.
func (g *GCSManager) Read(ctx context.Context, uri string) ([]byte, error) {
	cp := g.cachePath(uri)
	if data, err := os.ReadFile(cp); err == nil {
		return data, nil
	}

	bucket, object, err := ParseGCSURI(uri)
	if err != nil {
		return nil, err
	}

	reader, err := g.client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read gs://%s/%s: %w", bucket, object, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read gs://%s/%s: %w", bucket, object, err)
	}

	// Write to cache (best-effort).
	if err := os.MkdirAll(filepath.Dir(cp), 0o755); err == nil {
		_ = os.WriteFile(cp, data, 0o600)
	}

	return data, nil
}

// ExpandPattern lists GCS objects matching a gs:// glob pattern.
// Returns a list of full gs:// URIs that match.
func (g *GCSManager) ExpandPattern(ctx context.Context, pattern string) ([]string, error) {
	bucket, objectPattern, err := ParseGCSURI(pattern)
	if err != nil {
		return nil, err
	}

	// Extract prefix (everything before the first wildcard character).
	prefix := objectPattern
	for i, c := range objectPattern {
		if c == '*' || c == '?' || c == '[' {
			prefix = objectPattern[:i]
			break
		}
	}
	// Trim to last slash to get a directory prefix.
	if idx := strings.LastIndexByte(prefix, '/'); idx >= 0 {
		prefix = prefix[:idx+1]
	} else {
		prefix = ""
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	it := g.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})

	var uris []string
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects in gs://%s/%s: %w", bucket, prefix, err)
		}

		// Skip "directory" markers.
		if strings.HasSuffix(attrs.Name, "/") {
			continue
		}

		matched, err := doublestar.Match(objectPattern, attrs.Name)
		if err != nil {
			slog.Warn("glob match error", "pattern", objectPattern, "object", attrs.Name, "error", err)
			continue
		}
		if matched {
			uris = append(uris, fmt.Sprintf("gs://%s/%s", bucket, attrs.Name))
		}
	}

	return uris, nil
}

// InvalidateCache removes the cached content for a GCS URI.
func (g *GCSManager) InvalidateCache(uri string) {
	_ = os.Remove(g.cachePath(uri))
}

// Close closes the underlying GCS client.
func (g *GCSManager) Close() error {
	return g.client.Close()
}

// cachePath returns the local disk cache path for a GCS URI.
func (g *GCSManager) cachePath(uri string) string {
	bucket, object, _ := ParseGCSURI(uri)
	return filepath.Join(g.cacheDir, bucket, object)
}

// --- State methods for GCS ---

// SetupGCS initializes the GCS manager on the server state.
func (s *State) SetupGCS(ctx context.Context, project, cacheDir string, buckets map[string]string) error {
	mgr, err := NewGCSManager(ctx, project, cacheDir, buckets)
	if err != nil {
		return err
	}
	s.gcs = mgr
	return nil
}

// HasGCS returns true if a GCS manager is configured.
func (s *State) HasGCS() bool {
	return s.gcs != nil
}

// addGCSPattern registers a GCS glob pattern and expands it.
// On failure, records an error state and schedules a retry.
func (s *State) addGCSPattern(ctx context.Context, pattern, groupName string) ([]*FileEntry, error) {
	if s.gcs == nil {
		return nil, fmt.Errorf("GCS pattern %q but no gcs config provided", pattern)
	}

	// Register the pattern.
	added := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.patterns {
			if p.Pattern == pattern && p.Group == groupName {
				return false
			}
		}
		gp := &GlobPattern{
			Pattern:      pattern,
			PatternSlash: pattern, // already forward slashes
			Group:        groupName,
		}
		s.patterns = append(s.patterns, gp)
		if _, ok := s.groups[groupName]; !ok {
			s.groups[groupName] = &Group{Name: groupName}
		}
		return true
	}()
	if !added {
		return nil, nil
	}

	// Expand — calls GCS list API.
	uris, err := s.gcs.ExpandPattern(ctx, pattern)
	if err != nil {
		slog.Error("GCS pattern expansion failed", "pattern", pattern, "error", err)
		s.mu.Lock()
		s.gcsErrors[groupName] = gcsGroupError{
			Message: fmt.Sprintf("failed to list %s: %v", pattern, err),
			RetryAt: time.Now().Add(30 * time.Second),
		}
		s.mu.Unlock()
		s.scheduleGCSRetry(ctx, groupName, pattern, 30*time.Second)
		s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
		return nil, nil // don't fail the server
	}

	// Clear any previous error for this group.
	s.mu.Lock()
	delete(s.gcsErrors, groupName)
	s.mu.Unlock()

	var entries []*FileEntry
	for _, uri := range uris {
		entry, err := s.addGCSFile(uri, groupName)
		if err != nil {
			slog.Warn("skipping GCS file", "uri", uri, "error", err)
			continue
		}
		entries = append(entries, entry)
	}

	slog.Info("GCS pattern expanded", "pattern", pattern, "matched", len(entries))
	return entries, nil
}

// addGCSFile registers a GCS file in a group. No fsnotify — Pub/Sub handles changes.
func (s *State) addGCSFile(uri, groupName string) (*FileEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.groups[groupName]
	if !ok {
		g = &Group{Name: groupName}
		s.groups[groupName] = g
	}

	for _, f := range g.Files {
		if f.Path == uri {
			return f, nil
		}
	}

	_, object, _ := ParseGCSURI(uri)
	entry := &FileEntry{
		Name: filepath.Base(object),
		ID:   FileID(uri),
		Path: uri,
	}
	g.Files = append(g.Files, entry)

	slog.Info("GCS file added", "uri", uri, "group", groupName, "id", entry.ID)
	s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
	return entry, nil
}

// FindFileByPath looks up a file entry by its path (local or GCS URI).
func (s *State) FindFileByPath(path string) *FileEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.groups {
		for _, f := range g.Files {
			if f.Path == path {
				return f
			}
		}
	}
	return nil
}

// RemoveFileByPath removes a file from all groups by its path.
func (s *State) RemoveFileByPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.groups {
		for i, f := range g.Files {
			if f.Path == path {
				g.Files = append(g.Files[:i], g.Files[i+1:]...)
				s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
				return
			}
		}
	}
}

// scheduleGCSRetry schedules a retry for a failed GCS pattern expansion.
func (s *State) scheduleGCSRetry(ctx context.Context, groupName, pattern string, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if t, ok := s.gcsRetryTimers[groupName]; ok {
		t.Stop()
	}

	s.gcsRetryTimers[groupName] = time.AfterFunc(delay, func() {
		slog.Info("retrying GCS pattern expansion", "group", groupName, "pattern", pattern)
		_, _ = s.addGCSPattern(ctx, pattern, groupName)
	})
}
