# Plan: Add GCS with Pub/Sub Support

## Decisions

| # | Topic | Decision |
|---|-------|----------|
| 1 | Pub/Sub subscriptions | Pre-existing only. User creates topic, notification, and subscription. Moted consumes. |
| 2 | GCS file editing | Governed by existing `read-only` config flag. When not read-only, writes back to GCS. |
| 3 | Content delivery | Full download with disk cache (`$XDG_CACHE_HOME/moted/gcs/`). |
| 4 | `gcs.project` | Explicitly required when GCS buckets are configured. |
| 5 | Graceful degradation | Server starts, local groups work, failed GCS groups show error in UI, auto-retry every 30s. |
| 6 | GCS change detection | Pub/Sub only. No polling for GCS (too expensive — stat/list calls cost money). Polling stays for local non-inotify mounts only. |
| 7 | Config shape | Keep existing groups structure. `gs://` URI scheme distinguishes GCS from local. Bucket-level Pub/Sub config. |

## Config Format

```yaml
# Existing settings — unchanged
port: 8080
bind: localhost
read-only: false
poll-interval: 5s          # for local FUSE mounts only, not GCS

# NEW: GCS settings
gcs:
  project: my-gcp-project   # required, used for Pub/Sub client
  cache-dir: ""              # optional, defaults to $XDG_CACHE_HOME/moted/gcs/
  buckets:
    my-bucket:
      subscription: moted-my-bucket-sub
    other-bucket:
      subscription: moted-other-bucket-sub

# Groups — unchanged structure, gs:// URIs just work
groups:
  - name: local-docs
    watch:
      - ./**/*.md

  - name: reports
    watch:
      - gs://my-bucket/reports/**/*.md
      - gs://my-bucket/summaries/**/*.md

  - name: external
    watch:
      - gs://other-bucket/docs/**/*.md
```

## GCP Setup (user responsibility)

```bash
# Per bucket, one-time:

# 1. Create topic
gcloud pubsub topics create moted-my-bucket

# 2. Create notification on bucket → topic
gsutil notification create \
  -t moted-my-bucket \
  -f json \
  -e OBJECT_FINALIZE -e OBJECT_DELETE \
  gs://my-bucket

# 3. Create subscription for moted
gcloud pubsub subscriptions create moted-my-bucket-sub \
  --topic=moted-my-bucket
```

---

## Implementation Details

### New Go dependencies

```
cloud.google.com/go/storage    # GCS client
cloud.google.com/go/pubsub/v2  # Pub/Sub client
```

Both use Application Default Credentials automatically. No extra auth code needed beyond creating the clients.

---

### Phase 1: Config Parsing

**Modify `cmd/config.go`**

Add GCS config structs and wire them into `configFile`:

```go
// Add after groupConfig struct (line 35):

type gcsConfig struct {
    Project  string                  `yaml:"project"`
    CacheDir string                  `yaml:"cache-dir"`
    Buckets  map[string]bucketConfig `yaml:"buckets"`
}

type bucketConfig struct {
    Subscription string `yaml:"subscription"`
}
```

Add field to `configFile` struct (after line 28):

```go
type configFile struct {
    // ... existing fields ...
    PollInterval string        `yaml:"poll-interval"`
    GCS          *gcsConfig    `yaml:"gcs"`       // NEW
    Groups       []groupConfig `yaml:"groups"`
}
```

**Modify `cmd/config.go` — `buildGroupsFromConfig`** (line 98)

Currently `resolvePatterns` calls `filepath.Abs()` on patterns. For `gs://` patterns, skip the filepath resolution and pass them through as-is. Add a check:

```go
func isGCSPattern(pattern string) bool {
    return strings.HasPrefix(pattern, "gs://")
}
```

In `resolvePatterns`, if `isGCSPattern(p)`, append it directly without calling `filepath.Abs`.

**Modify `cmd/root.go` — `startServer`** (around line 1072)

After creating the server `State`, if `cfg.GCS != nil`:
1. Validate that `cfg.GCS.Project` is non-empty
2. Validate that every bucket referenced in `gs://` watch patterns has a corresponding entry in `cfg.GCS.Buckets`
3. Pass the GCS config to a new `state.SetupGCS(ctx, gcsConf)` method

---

### Phase 2: GCS Source (`internal/server/gcs.go` — new file)

Create `internal/server/gcs.go` with:

```go
package server

import (
    "cloud.google.com/go/storage"
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/bmatcuk/doublestar/v4"
)

// ParseGCSURI parses "gs://bucket/path/to/object" into bucket and object.
// Returns ("", "", error) if the URI doesn't start with "gs://".
func ParseGCSURI(uri string) (bucket, object string, err error) {
    if !strings.HasPrefix(uri, "gs://") {
        return "", "", fmt.Errorf("not a GCS URI: %s", uri)
    }
    rest := strings.TrimPrefix(uri, "gs://")
    idx := strings.IndexByte(rest, '/')
    if idx < 0 {
        return rest, "", nil
    }
    return rest[:idx], rest[idx+1:], nil
}

// IsGCSPath returns true if the path starts with "gs://".
func IsGCSPath(path string) bool {
    return strings.HasPrefix(path, "gs://")
}

// GCSManager handles GCS operations for all configured buckets.
type GCSManager struct {
    client   *storage.Client
    project  string
    cacheDir string                  // disk cache root
    buckets  map[string]bucketState  // bucket name → state
}

type bucketState struct {
    subscription string
}

// NewGCSManager creates a GCS manager. Uses Application Default Credentials.
func NewGCSManager(ctx context.Context, project, cacheDir string, buckets map[string]string) (*GCSManager, error) {
    client, err := storage.NewClient(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCS client: %w", err)
    }
    bs := make(map[string]bucketState, len(buckets))
    for name, sub := range buckets {
        bs[name] = bucketState{subscription: sub}
    }
    return &GCSManager{
        client:   client,
        project:  project,
        cacheDir: cacheDir,
        buckets:  bs,
    }, nil
}

// Read downloads a GCS object. Checks disk cache first.
// uri is "gs://bucket/object/path".
func (g *GCSManager) Read(ctx context.Context, uri string) ([]byte, error) {
    // 1. Check disk cache: g.cachePath(uri)
    // 2. If cache hit, return cached content
    // 3. If cache miss, download from GCS, write to cache, return
}

// Write uploads content to a GCS object and updates the disk cache.
func (g *GCSManager) Write(ctx context.Context, uri string, data []byte) error {
    // 1. Upload to GCS via bucket.Object(object).NewWriter(ctx)
    // 2. Update disk cache
}

// ExpandPattern lists GCS objects matching a gs:// glob pattern.
// Returns a list of full gs:// URIs.
// Example: "gs://my-bucket/reports/**/*.md" →
//   ["gs://my-bucket/reports/2026/jan.md", "gs://my-bucket/reports/2026/feb.md"]
func (g *GCSManager) ExpandPattern(ctx context.Context, pattern string) ([]string, error) {
    bucket, objectPattern, err := ParseGCSURI(pattern)
    if err != nil {
        return nil, err
    }
    // 1. Determine prefix from objectPattern (everything before the first wildcard)
    //    e.g., "reports/**/*.md" → prefix "reports/"
    // 2. List objects with that prefix: g.client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
    // 3. For each object, check if it matches the full glob pattern using doublestar.Match
    //    doublestar.Match(objectPattern, obj.Name)
    // 4. Return matching URIs as "gs://bucket/object"
}

// InvalidateCache removes the cached content for a GCS URI.
func (g *GCSManager) InvalidateCache(uri string) error {
    path := g.cachePath(uri)
    return os.Remove(path)
}

// cachePath returns the local disk path for a GCS URI.
// "gs://my-bucket/reports/file.md" → "<cacheDir>/my-bucket/reports/file.md"
func (g *GCSManager) cachePath(uri string) string {
    bucket, object, _ := ParseGCSURI(uri)
    return filepath.Join(g.cacheDir, bucket, object)
}

// Close closes the underlying GCS client.
func (g *GCSManager) Close() error {
    return g.client.Close()
}
```

**Cache directory resolution** — add to `internal/xdg/xdg.go`:

```go
// CacheHome returns $XDG_CACHE_HOME, defaulting to ~/.cache.
func CacheHome() (string, error) {
    if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
        return v, nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    return filepath.Join(home, ".cache"), nil
}
```

Default cache dir: `filepath.Join(xdg.CacheHome(), "moted", "gcs")`.

---

### Phase 3: Wire GCS into Server State

**Modify `internal/server/server.go` — `State` struct** (line 73)

Add these fields:

```go
type State struct {
    // ... existing fields (lines 74-99) ...

    // GCS support
    gcs            *GCSManager              // nil when no GCS configured
    gcsErrors      map[string]gcsGroupError // group name → error state
    gcsRetryTimers map[string]*time.Timer   // group name → retry timer
    pendingWrites  map[string]struct{}       // GCS URIs with in-flight writes (for echo suppression)
}
```

Add the error struct:

```go
type gcsGroupError struct {
    Message string    `json:"error"`
    RetryAt time.Time `json:"retryAt"`
}
```

**Modify `NewState`** (line 110) — initialize the new maps:

```go
gcsErrors:      make(map[string]gcsGroupError),
gcsRetryTimers: make(map[string]*time.Timer),
pendingWrites:  make(map[string]struct{}),
```

**New method `SetupGCS` on State** — add to `internal/server/gcs.go`:

```go
func (s *State) SetupGCS(ctx context.Context, project, cacheDir string, buckets map[string]string) error {
    mgr, err := NewGCSManager(ctx, project, cacheDir, buckets)
    if err != nil {
        return err
    }
    s.gcs = mgr
    return nil
}
```

---

### Phase 4: GCS Pattern Expansion on Startup

**Modify `State.AddPattern`** (`server.go` line 548)

Currently `AddPattern` assumes local filesystem:
- Line 559: `os.Stat(base)` to validate base dir exists
- Line 594: `doublestar.Glob(os.DirFS(base), relPat)` to expand
- Line 612: `s.watchDirsForPattern(gp)` to set up fsnotify

For GCS patterns, the flow is different. Add a branch at the top of `AddPattern`:

```go
func (s *State) AddPattern(absPattern, groupName string) ([]*FileEntry, error) {
    if IsGCSPath(absPattern) {
        return s.addGCSPattern(absPattern, groupName)
    }
    // ... existing local logic unchanged ...
}
```

**New method `addGCSPattern`**:

```go
func (s *State) addGCSPattern(pattern, groupName string) ([]*FileEntry, error) {
    if s.gcs == nil {
        return nil, fmt.Errorf("GCS pattern %q but no gcs config provided", pattern)
    }

    // Register the pattern (same lock dance as AddPattern)
    s.mu.Lock()
    for _, p := range s.patterns {
        if p.Pattern == pattern && p.Group == groupName {
            s.mu.Unlock()
            return nil, nil
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
    s.mu.Unlock()

    // Expand — this calls GCS list API
    uris, err := s.gcs.ExpandPattern(context.Background(), pattern)
    if err != nil {
        // Record error state, schedule retry
        s.mu.Lock()
        s.gcsErrors[groupName] = gcsGroupError{
            Message: fmt.Sprintf("failed to list %s: %v", pattern, err),
            RetryAt: time.Now().Add(30 * time.Second),
        }
        s.mu.Unlock()
        s.scheduleGCSRetry(groupName, pattern, 30*time.Second)
        s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
        return nil, nil // don't fail the server
    }

    var entries []*FileEntry
    for _, uri := range uris {
        entry, err := s.addGCSFile(uri, groupName)
        if err != nil {
            slog.Warn("skipping GCS file", "uri", uri, "error", err)
            continue
        }
        entries = append(entries, entry)
    }
    return entries, nil
}
```

**New method `addGCSFile`** — similar to `AddFile` but for GCS URIs:

```go
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
        ID:   FileID(uri),      // SHA-256 of "gs://bucket/path"
        Path: uri,              // Store full GCS URI in Path field
    }
    g.Files = append(g.Files, entry)

    // No fsnotify watch — Pub/Sub handles change detection
    slog.Debug("GCS file added", "uri", uri, "group", groupName, "id", entry.ID)
    s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
    return entry, nil
}
```

---

### Phase 5: Serve GCS File Content

**Modify `handleFileContent`** (`server.go` line 1672)

Currently reads local files with `os.ReadFile` at line 1693. Add a GCS branch:

```go
// Replace the else branch at line 1692:
} else if IsGCSPath(entry.Path) {
    content, err := state.gcs.Read(r.Context(), entry.Path)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    resp = fileContentResponse{
        Content: string(content),
        BaseDir: "", // no local base dir for GCS files
    }
} else {
    // existing os.ReadFile logic
}
```

**Same change needed in `handleFileTextRaw`** (line 1710) and **`handleFileRaw`** (line 1746) — these also read from `os.ReadFile`.

---

### Phase 6: Pub/Sub Subscriber (`internal/server/pubsub.go` — new file)

```go
package server

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "strings"

    "cloud.google.com/go/pubsub/v2"
    "github.com/bmatcuk/doublestar/v4"
    "github.com/k1LoW/donegroup"
)

// StartPubSubSubscribers starts a goroutine for each bucket subscription.
// Call after SetupGCS and after all patterns have been added.
func (s *State) StartPubSubSubscribers(ctx context.Context) error {
    if s.gcs == nil {
        return nil
    }

    client, err := pubsub.NewClient(ctx, s.gcs.project)
    if err != nil {
        return fmt.Errorf("failed to create Pub/Sub client: %w", err)
    }

    for bucketName, bs := range s.gcs.buckets {
        if bs.subscription == "" {
            continue
        }
        sub := client.Subscription(bs.subscription)
        bucket := bucketName // capture for goroutine

        donegroup.Go(ctx, func() error {
            slog.Info("starting Pub/Sub subscriber",
                "bucket", bucket, "subscription", bs.subscription)
            // sub.Receive blocks until ctx is cancelled
            return sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
                s.handlePubSubMessage(bucket, msg)
                msg.Ack()
            })
        })
    }
    return nil
}
```

**Pub/Sub message handling:**

GCS notification messages have these attributes:
- `bucketId`: bucket name
- `objectId`: object path (e.g., "reports/2026/file.md")
- `eventType`: "OBJECT_FINALIZE" (create/update) or "OBJECT_DELETE"
- `objectGeneration`: generation number

```go
func (s *State) handlePubSubMessage(bucket string, msg *pubsub.Message) {
    objectID := msg.Attributes["objectId"]
    eventType := msg.Attributes["eventType"]
    if objectID == "" || eventType == "" {
        slog.Warn("ignoring Pub/Sub message with missing attributes",
            "bucket", bucket, "attributes", msg.Attributes)
        return
    }

    uri := fmt.Sprintf("gs://%s/%s", bucket, objectID)

    // Check if this is a self-originated write — skip if so
    s.mu.Lock()
    if _, pending := s.pendingWrites[uri]; pending {
        delete(s.pendingWrites, uri)
        s.mu.Unlock()
        slog.Debug("ignoring self-originated Pub/Sub event", "uri", uri)
        return
    }
    s.mu.Unlock()

    // Match against all GCS watch patterns
    matched := false
    s.mu.RLock()
    for _, gp := range s.patterns {
        if !IsGCSPath(gp.Pattern) {
            continue
        }
        ok, _ := doublestar.Match(gp.PatternSlash, uri)
        if ok {
            matched = true
            break
        }
    }
    s.mu.RUnlock()

    if !matched {
        return // object doesn't match any watch pattern
    }

    switch eventType {
    case "OBJECT_FINALIZE":
        // File created or updated
        s.gcs.InvalidateCache(uri)

        // Add if new, or notify if existing
        entry := s.FindFileByPath(uri)
        if entry == nil {
            // New file — find which group(s) it belongs to
            s.mu.RLock()
            for _, gp := range s.patterns {
                if !IsGCSPath(gp.Pattern) {
                    continue
                }
                if ok, _ := doublestar.Match(gp.PatternSlash, uri); ok {
                    s.mu.RUnlock()
                    s.addGCSFile(uri, gp.Group)
                    s.mu.RLock()
                }
            }
            s.mu.RUnlock()
        } else {
            s.scheduleFileChanged(uri)
        }

    case "OBJECT_DELETE":
        // File deleted — remove from all groups
        s.RemoveFileByPath(uri)
    }
}
```

**New helper `FindFileByPath`** — add to `server.go`:

```go
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
```

**New helper `RemoveFileByPath`** — add to `server.go`:

```go
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
```

---

### Phase 7: Error State & Auto-Retry

**Retry mechanism:**

```go
func (s *State) scheduleGCSRetry(groupName, pattern string, delay time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Cancel existing timer if any
    if t, ok := s.gcsRetryTimers[groupName]; ok {
        t.Stop()
    }

    s.gcsRetryTimers[groupName] = time.AfterFunc(delay, func() {
        slog.Info("retrying GCS pattern expansion", "group", groupName, "pattern", pattern)
        entries, err := s.addGCSPattern(pattern, groupName) // will set error again if still failing
        if err == nil && entries != nil {
            // Success — clear error
            s.mu.Lock()
            delete(s.gcsErrors, groupName)
            delete(s.gcsRetryTimers, groupName)
            s.mu.Unlock()
            s.sendEvent(sseEvent{Name: eventUpdate, Data: "{}"})
        }
    })
}
```

**Expose error state in API response.**

Modify `Group` struct (`server.go` line 45):

```go
type Group struct {
    Name    string       `json:"name"`
    Files   []*FileEntry `json:"files"`
    Error   string       `json:"error,omitempty"`    // NEW
    RetryAt string       `json:"retryAt,omitempty"`  // NEW — RFC3339
}
```

Modify `handleGroups` (`server.go` line 1648) — after enriching file entries, overlay error state:

```go
// After the existing enrichment loop, add:
state.mu.RLock()
for i := range groups {
    if e, ok := state.gcsErrors[groups[i].Name]; ok {
        groups[i].Error = e.Message
        groups[i].RetryAt = e.RetryAt.UTC().Format(time.RFC3339)
    }
}
state.mu.RUnlock()
```

---

### Phase 8: Write-Back

**Modify `handleFileContent`** — currently there is no save/write handler. Need to add one.

Add a new route in `NewHandler` (after line 1477):

```go
mux.HandleFunc("PUT /_/api/files/{id}/content", handleSaveFile(state))
```

Implement `handleSaveFile`:

```go
func handleSaveFile(state *State) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        id := r.PathValue("id")
        entry := state.FindFile(id)
        if entry == nil {
            http.Error(w, "file not found", http.StatusNotFound)
            return
        }

        var body struct {
            Content string `json:"content"`
        }
        if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
            http.Error(w, "invalid JSON", http.StatusBadRequest)
            return
        }

        if IsGCSPath(entry.Path) {
            if state.gcs == nil {
                http.Error(w, "GCS not configured", http.StatusInternalServerError)
                return
            }
            // Mark as pending write to suppress the echo Pub/Sub event
            state.mu.Lock()
            state.pendingWrites[entry.Path] = struct{}{}
            state.mu.Unlock()

            if err := state.gcs.Write(r.Context(), entry.Path, []byte(body.Content)); err != nil {
                // Clean up pending write on failure
                state.mu.Lock()
                delete(state.pendingWrites, entry.Path)
                state.mu.Unlock()
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
        } else {
            if err := os.WriteFile(entry.Path, []byte(body.Content), 0644); err != nil {
                http.Error(w, err.Error(), http.StatusInternalServerError)
                return
            }
        }

        w.WriteHeader(http.StatusNoContent)
    }
}
```

Note: the `read-only` flag check should be done in middleware or at the handler level. Check how the existing code handles read-only mode and follow the same pattern.

---

### Phase 9: Frontend Changes

**Modify `internal/frontend/src/hooks/useApi.ts`**

Add error fields to `Group` interface (line 9):

```typescript
export interface Group {
  name: string;
  files: FileEntry[];
  error?: string;     // NEW
  retryAt?: string;   // NEW — ISO 8601
}
```

Add a `saveFileContent` function:

```typescript
export async function saveFileContent(id: string, content: string): Promise<void> {
  const res = await fetch(`/_/api/files/${id}/content`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) throw new Error("Failed to save file content");
}
```

**Modify `internal/frontend/src/components/Sidebar.tsx`**

When rendering a group's file list, check for `group.error`. If present, render an error banner instead of the file list:

```tsx
{group.error ? (
  <div className="p-3 text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/20 rounded">
    <p className="font-medium">Failed to load files</p>
    <p className="mt-1 text-xs opacity-80">{group.error}</p>
    {group.retryAt && (
      <p className="mt-1 text-xs opacity-60">
        Retrying at {new Date(group.retryAt).toLocaleTimeString()}
      </p>
    )}
  </div>
) : (
  /* existing file list rendering */
)}
```

---

### Phase 10: Startup Wiring (`cmd/root.go`)

In `startServer` (around line 1072), after `state := server.NewState(ctx)` and after applying config:

```go
// After existing pattern setup, add GCS initialization:
if cfg != nil && cfg.GCS != nil {
    // Resolve cache dir
    cacheDir := cfg.GCS.CacheDir
    if cacheDir == "" {
        cacheHome, err := xdg.CacheHome()
        if err != nil {
            return fmt.Errorf("failed to resolve cache home: %w", err)
        }
        cacheDir = filepath.Join(cacheHome, "moted", "gcs")
    }
    if err := os.MkdirAll(cacheDir, 0755); err != nil {
        return fmt.Errorf("failed to create GCS cache dir: %w", err)
    }

    // Build bucket → subscription map
    buckets := make(map[string]string)
    for name, bc := range cfg.GCS.Buckets {
        buckets[name] = bc.Subscription
    }

    if err := state.SetupGCS(ctx, cfg.GCS.Project, cacheDir, buckets); err != nil {
        return fmt.Errorf("failed to initialize GCS: %w", err)
    }

    // Start Pub/Sub subscribers (after patterns are added, so they can match)
    // This is called after the pattern loop below
}

// ... existing pattern loop adds both local and GCS patterns via AddPattern ...

// After pattern loop:
if state.HasGCS() {
    if err := state.StartPubSubSubscribers(ctx); err != nil {
        slog.Error("failed to start Pub/Sub subscribers", "error", err)
        // Non-fatal — GCS groups will show error state
    }
}
```

---

### Phase 11: Testing

**Unit tests for `ParseGCSURI`** — `internal/server/gcs_test.go`:
- `"gs://bucket/path/file.md"` → `("bucket", "path/file.md", nil)`
- `"gs://bucket"` → `("bucket", "", nil)`
- `"/local/path"` → error

**Unit tests for `GCSManager.ExpandPattern`** — use `storage.Client` with a fake or the GCS emulator (`STORAGE_EMULATOR_HOST`).

**Unit tests for `handlePubSubMessage`** — create a `State` with known patterns and mock `GCSManager`, send fake messages, verify files are added/removed and SSE events fire.

**Integration test** — use `PUBSUB_EMULATOR_HOST` and `STORAGE_EMULATOR_HOST` for a full end-to-end test.

---

### New files summary

| File | Purpose |
|------|---------|
| `internal/server/gcs.go` | `GCSManager`, `ParseGCSURI`, `IsGCSPath`, GCS file operations, disk cache, pattern expansion |
| `internal/server/pubsub.go` | `StartPubSubSubscribers`, `handlePubSubMessage`, Pub/Sub → SSE bridge |
| `internal/server/gcs_test.go` | Tests for GCS parsing, expansion, Pub/Sub handling |

### Modified files summary

| File | Changes |
|------|---------|
| `cmd/config.go` | Add `gcsConfig`, `bucketConfig` structs; update `configFile`; handle `gs://` in `resolvePatterns` |
| `cmd/root.go` | GCS initialization in `startServer`; Pub/Sub startup after patterns |
| `internal/server/server.go` | Add GCS fields to `State`; `addGCSFile`; `FindFileByPath`; `RemoveFileByPath`; branch in `AddPattern` for GCS; `Error`/`RetryAt` fields on `Group`; GCS branch in `handleFileContent`/`handleFileTextRaw`/`handleFileRaw`; new `handleSaveFile` route; error overlay in `handleGroups` |
| `internal/xdg/xdg.go` | Add `CacheHome()` |
| `internal/frontend/src/hooks/useApi.ts` | Add `error`/`retryAt` to `Group` interface; add `saveFileContent` |
| `internal/frontend/src/components/Sidebar.tsx` | Render error state for GCS groups |
| `go.mod` | Add `cloud.google.com/go/storage`, `cloud.google.com/go/pubsub/v2` |
| `docs/config.example.yaml` | Add GCS config example |
