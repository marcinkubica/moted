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
		shutdownCh:  make(chan struct{}, 1),
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

func TestMoveFile(t *testing.T) {
	t.Run("moves file to existing group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}, {ID: 2, Name: "b.md", Path: "/b.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: 3, Name: "c.md", Path: "/c.md"}},
		}

		if err := s.MoveFile(1, "dst"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if len(s.groups["src"].Files) != 1 || s.groups["src"].Files[0].ID != 2 {
			t.Error("source group should have only file 2")
		}
		if len(s.groups["dst"].Files) != 2 || s.groups["dst"].Files[1].ID != 1 {
			t.Error("target group should have file 1 appended")
		}
	})

	t.Run("auto-creates target group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}, {ID: 2, Name: "b.md", Path: "/b.md"}},
		}

		if err := s.MoveFile(1, "newgroup"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if _, ok := s.groups["newgroup"]; !ok {
			t.Fatal("target group should have been created")
		}
		if s.groups["newgroup"].Files[0].ID != 1 {
			t.Error("target group should contain file 1")
		}
	})

	t.Run("deletes empty source group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: 2, Name: "b.md", Path: "/b.md"}},
		}

		if err := s.MoveFile(1, "dst"); err != nil {
			t.Fatalf("MoveFile returned error: %v", err)
		}

		if _, ok := s.groups["src"]; ok {
			t.Error("empty source group should have been deleted")
		}
	})

	t.Run("returns error for duplicate path", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: 2, Name: "a.md", Path: "/a.md"}},
		}

		err := s.MoveFile(1, "dst")
		if err == nil {
			t.Fatal("MoveFile should return error for duplicate path")
		}
	})

	t.Run("returns error for same group", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}},
		}

		err := s.MoveFile(1, "src")
		if err == nil {
			t.Fatal("MoveFile should return error for same group")
		}
	})

	t.Run("returns error for unknown file", func(t *testing.T) {
		s := newTestState(t)
		err := s.MoveFile(999, "dst")
		if err == nil {
			t.Fatal("MoveFile should return error for unknown file")
		}
	})
}

func TestHandleMoveFile(t *testing.T) {
	t.Run("moves file via HTTP", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: 2, Name: "b.md", Path: "/b.md"}},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(moveFileRequest{Group: "dst"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/files/1/group", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
		}
		if len(s.groups["dst"].Files) != 2 {
			t.Error("target group should have 2 files")
		}
	})

	t.Run("returns 409 for duplicate path", func(t *testing.T) {
		s := newTestState(t)
		s.groups["src"] = &Group{
			Name:  "src",
			Files: []*FileEntry{{ID: 1, Name: "a.md", Path: "/a.md"}},
		}
		s.groups["dst"] = &Group{
			Name:  "dst",
			Files: []*FileEntry{{ID: 2, Name: "a.md", Path: "/a.md"}},
		}

		handler := NewHandler(s)
		body, err := json.Marshal(moveFileRequest{Group: "dst"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("PUT", "/_/api/files/1/group", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusConflict)
		}
	})
}

func TestHandleShutdown(t *testing.T) {
	t.Run("returns 202 and signals shutdownCh", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)
		req := httptest.NewRequest("POST", "/_/api/shutdown", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("got status %d, want %d", rec.Code, http.StatusAccepted)
		}

		select {
		case <-s.ShutdownCh():
		default:
			t.Fatal("shutdownCh should have received a signal")
		}
	})

	t.Run("does not block on duplicate signal", func(t *testing.T) {
		s := newTestState(t)
		handler := NewHandler(s)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("POST", "/_/api/shutdown", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("call %d: got status %d, want %d", i+1, rec.Code, http.StatusAccepted)
			}
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
