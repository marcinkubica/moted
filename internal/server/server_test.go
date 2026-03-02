package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestState(t *testing.T) *State {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	s := &State{
		groups:      make(map[string]*Group),
		nextID:      1,
		subscribers: make(map[chan sseEvent]struct{}),
		restartCh:   make(chan string, 1),
	}
	_ = ctx
	return s
}

func TestReorderFiles(t *testing.T) {
	t.Run("reorders files successfully", func(t *testing.T) {
		s := newTestState(t)
		s.groups["default"] = &Group{
			Name: "default",
			Files: []*FileEntry{
				{ID: 1, Name: "a.md", Path: "/a.md"},
				{ID: 2, Name: "b.md", Path: "/b.md"},
				{ID: 3, Name: "c.md", Path: "/c.md"},
			},
		}

		ok := s.ReorderFiles("default", []int{3, 1, 2})
		if !ok {
			t.Fatal("ReorderFiles returned false, want true")
		}

		files := s.groups["default"].Files
		if files[0].ID != 3 || files[1].ID != 1 || files[2].ID != 2 {
			t.Errorf("got order [%d, %d, %d], want [3, 1, 2]", files[0].ID, files[1].ID, files[2].ID)
		}
	})

	t.Run("returns false for unknown group", func(t *testing.T) {
		s := newTestState(t)
		ok := s.ReorderFiles("nonexistent", []int{1})
		if ok {
			t.Fatal("ReorderFiles returned true for unknown group")
		}
	})

	t.Run("returns false for mismatched count", func(t *testing.T) {
		s := newTestState(t)
		s.groups["default"] = &Group{
			Name: "default",
			Files: []*FileEntry{
				{ID: 1, Name: "a.md", Path: "/a.md"},
				{ID: 2, Name: "b.md", Path: "/b.md"},
			},
		}

		ok := s.ReorderFiles("default", []int{1})
		if ok {
			t.Fatal("ReorderFiles returned true for mismatched count")
		}
	})

	t.Run("returns false for unknown file ID", func(t *testing.T) {
		s := newTestState(t)
		s.groups["default"] = &Group{
			Name: "default",
			Files: []*FileEntry{
				{ID: 1, Name: "a.md", Path: "/a.md"},
				{ID: 2, Name: "b.md", Path: "/b.md"},
			},
		}

		ok := s.ReorderFiles("default", []int{1, 99})
		if ok {
			t.Fatal("ReorderFiles returned true for unknown file ID")
		}
	})
}

func TestHandleReorderFiles(t *testing.T) {
	t.Run("reorders files via HTTP", func(t *testing.T) {
		s := newTestState(t)
		s.groups["docs"] = &Group{
			Name: "docs",
			Files: []*FileEntry{
				{ID: 1, Name: "a.md", Path: "/a.md"},
				{ID: 2, Name: "b.md", Path: "/b.md"},
			},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(reorderFilesRequest{FileIDs: []int{2, 1}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/groups/docs/order", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
		}

		files := s.groups["docs"].Files
		if files[0].ID != 2 || files[1].ID != 1 {
			t.Errorf("got order [%d, %d], want [2, 1]", files[0].ID, files[1].ID)
		}
	})

	t.Run("returns 400 for invalid group", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)
		body, err := json.Marshal(reorderFilesRequest{FileIDs: []int{1}})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/groups/nonexistent/order", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("returns 400 for invalid JSON", func(t *testing.T) {
		s := newTestState(t)
		s.groups["default"] = &Group{Name: "default", Files: []*FileEntry{}}
		handler := NewHandler(s)
		req := httptest.NewRequest("PUT", "/_/api/groups/default/order", bytes.NewReader([]byte("invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}
