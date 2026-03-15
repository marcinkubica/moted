# Security Review

Date: 2026-03-15
Repository: mo
Branch: few-things

## Scope

This review focused on the primary security boundaries in the repository:

- CLI inputs and bind behavior
- HTTP API handlers
- File access and path resolution
- Frontend markdown rendering and browser-origin behavior
- SSE and local service exposure

## Findings

### 1. Critical: Relative file opening is path-traversable

Location: [internal/server/server.go](../internal/server/server.go#L1310)

The relative file open handler joins the current file directory with a user-controlled path:

```go
absPath := filepath.Join(filepath.Dir(entry.Path), req.Path)
absPath = filepath.Clean(absPath)
```

There is no containment check before `os.Stat` and `state.AddFile`. A malicious markdown link or direct API call can use paths like `../../../../.ssh/config` to make mo read any file accessible to the local user.

Relevant frontend call sites:

- [internal/frontend/src/hooks/useApi.ts](../internal/frontend/src/hooks/useApi.ts#L35)
- [internal/frontend/src/components/MarkdownViewer.tsx](../internal/frontend/src/components/MarkdownViewer.tsx#L461)

Impact:

- Arbitrary local file disclosure within the permissions of the user running mo

### 2. Critical: Raw asset endpoint has a broken directory-boundary check

Location: [internal/server/server.go](../internal/server/server.go#L1276)

The raw file handler attempts to block traversal using a string prefix check:

```go
baseDir := filepath.Dir(entry.Path)
if !strings.HasPrefix(absPath, baseDir) {
	http.Error(w, "access denied", http.StatusForbidden)
	return
}
```

This is not a safe path-boundary test. For example, if `baseDir` is `/tmp/docs`, then `/tmp/docs-secret` still passes `HasPrefix`, even though it is outside the intended directory.

Relevant frontend path rewriting:

- [internal/frontend/src/utils/resolve.ts](../internal/frontend/src/utils/resolve.ts#L26)
- [internal/frontend/src/components/MarkdownViewer.tsx](../internal/frontend/src/components/MarkdownViewer.tsx#L458)

Impact:

- Arbitrary local file disclosure through crafted image or file links in markdown

### 3. High: Untrusted markdown is rendered with raw HTML enabled

Location: [internal/frontend/src/components/MarkdownViewer.tsx](../internal/frontend/src/components/MarkdownViewer.tsx#L521)

Markdown rendering includes `rehypeRaw`:

```tsx
rehypePlugins={[rehypeRaw, rehypeGithubAlerts, rehypeSlug, rehypeKatex]}
```

The CLI also explicitly documents raw HTML support:

- [cmd/root.go](../cmd/root.go#L128)

This means markdown is not treated as inert content. If untrusted markdown is opened, active HTML is introduced into the mo origin. Depending on browser behavior and allowed HTML constructs, that can enable script execution or origin-level interaction with the local API.

Impact:

- XSS-style behavior in the mo origin
- Amplifies the impact of local API exposure

### 4. Medium: Restart and shutdown endpoints are unauthenticated and origin-blind

Locations:

- [internal/server/server.go](../internal/server/server.go#L1081)
- [internal/server/server.go](../internal/server/server.go#L1082)
- [internal/server/server.go](../internal/server/server.go#L1408)
- [internal/server/server.go](../internal/server/server.go#L1427)

The restart and shutdown handlers accept POST requests without checking origin, referrer, or any CSRF token.

Impact:

- Any webpage the user visits while mo is running can attempt a blind POST to localhost and restart or shut the service down
- This is at minimum a browser-driven denial of service risk

### 5. Medium: Remote-access mode exposes full unauthenticated administration

Locations:

- [cmd/root.go](../cmd/root.go#L174)
- [cmd/root.go](../cmd/root.go#L323)
- [cmd/root.go](../cmd/root.go#L324)
- [cmd/root.go](../cmd/root.go#L325)

The `--dangerously-allow-remote-access` flag intentionally permits unauthenticated remote use. The warning text correctly states that remote clients can:

- Read any file accessible by this user
- Browse the filesystem via glob patterns
- Shut down or restart the server

This is disclosed behavior, but it remains a severe operational risk on shared or semi-trusted networks.

## Test Coverage Observed

The current test suite covers uploaded-file restrictions around relative opening, but I did not find regression coverage for traversal attempts in the raw or open handlers.

Relevant existing test reference:

- [internal/server/server_test.go](../internal/server/server_test.go#L1385)

## Recommended Fixes

1. Replace string-prefix containment checks with a path-aware validation strategy based on `filepath.Rel` and explicit rejection of any path that escapes the base directory.
2. Add the same containment validation to both the raw-file and relative-open handlers.
3. Add regression tests for `..` traversal, sibling-prefix bypasses, and encoded traversal attempts.
4. Revisit the trust model for raw HTML rendering. If untrusted markdown is in scope, sanitize or disable raw HTML by default.
5. Add origin validation or CSRF protection to state-changing localhost endpoints such as restart and shutdown.
6. Consider authentication or a capability token for remote-access mode.

## Notes

This review was based on static analysis of the repository and existing tests. I did not run exploit proofs or modify application code as part of this document creation.