package server

import (
	"testing"

	"github.com/bmatcuk/doublestar/v4"
)

func TestParseGCSURI(t *testing.T) {
	tests := []struct {
		uri        string
		wantBucket string
		wantObject string
		wantErr    bool
	}{
		{"gs://my-bucket/reports/file.md", "my-bucket", "reports/file.md", false},
		{"gs://my-bucket/path/to/deep/file.md", "my-bucket", "path/to/deep/file.md", false},
		{"gs://my-bucket", "my-bucket", "", false},
		{"gs://my-bucket/", "my-bucket", "", false},
		{"/local/path/file.md", "", "", true},
		{"", "", "", true},
		{"s3://bucket/key", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			bucket, object, err := ParseGCSURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseGCSURI(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
				return
			}
			if bucket != tt.wantBucket {
				t.Errorf("ParseGCSURI(%q) bucket = %q, want %q", tt.uri, bucket, tt.wantBucket)
			}
			if object != tt.wantObject {
				t.Errorf("ParseGCSURI(%q) object = %q, want %q", tt.uri, object, tt.wantObject)
			}
		})
	}
}

func TestIsGCSPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"gs://bucket/path", true},
		{"gs://bucket", true},
		{"/local/path", false},
		{"./relative", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsGCSPath(tt.path); got != tt.want {
				t.Errorf("IsGCSPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFileID_GCS(t *testing.T) {
	// GCS URIs should produce deterministic IDs.
	uri := "gs://my-bucket/reports/file.md"
	id1 := FileID(uri)
	id2 := FileID(uri)
	if id1 != id2 {
		t.Errorf("FileID(%q) not deterministic: %q != %q", uri, id1, id2)
	}
	if len(id1) != 8 {
		t.Errorf("FileID(%q) length = %d, want 8", uri, len(id1))
	}

	// Different URIs should (almost certainly) produce different IDs.
	other := FileID("gs://my-bucket/reports/other.md")
	if id1 == other {
		t.Errorf("FileID collision: %q and %q both produce %q", uri, "gs://my-bucket/reports/other.md", id1)
	}
}

func TestPubSubMessageMatching(t *testing.T) {
	// Test that handlePubSubMessage correctly matches patterns.
	// We can't easily test the full Pub/Sub flow without an emulator,
	// but we can test the pattern matching logic used within it.

	tests := []struct {
		pattern string
		uri     string
		want    bool
	}{
		{"gs://bucket/reports/**/*.md", "gs://bucket/reports/2026/file.md", true},
		{"gs://bucket/reports/**/*.md", "gs://bucket/reports/file.md", true},
		{"gs://bucket/reports/**/*.md", "gs://bucket/other/file.md", false},
		{"gs://bucket/reports/**/*.md", "gs://bucket/reports/file.txt", false},
		{"gs://bucket/*.md", "gs://bucket/file.md", true},
		{"gs://bucket/*.md", "gs://bucket/sub/file.md", false},
		{"gs://other-bucket/**/*.md", "gs://bucket/file.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.pattern+"→"+tt.uri, func(t *testing.T) {
			// Use doublestar.Match directly, same as handlePubSubMessage.
			got, err := matchGCSPattern(tt.pattern, tt.uri)
			if err != nil {
				t.Fatalf("matchGCSPattern(%q, %q) error: %v", tt.pattern, tt.uri, err)
			}
			if got != tt.want {
				t.Errorf("matchGCSPattern(%q, %q) = %v, want %v", tt.pattern, tt.uri, got, tt.want)
			}
		})
	}
}

// matchGCSPattern is a test helper that mirrors the pattern matching logic
// used in handlePubSubMessage.
func matchGCSPattern(pattern, uri string) (bool, error) {
	if !IsGCSPath(pattern) || !IsGCSPath(uri) {
		return false, nil
	}
	// Both pattern and uri use forward slashes already.
	return doublestarMatch(pattern, uri)
}

// doublestarMatch wraps doublestar.Match for testing.
func doublestarMatch(pattern, name string) (bool, error) {
	return doublestar.Match(pattern, name)
}
