package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zinrai/metal-install/internal/data"
	"github.com/zinrai/metal-install/internal/datadir"
)

// TestHandleCreateNode_InvalidNodeID verifies that POST /nodes rejects
// a node_id that is empty or would resolve outside the state
// directory, and writes nothing in that case.
func TestHandleCreateNode_InvalidNodeID(t *testing.T) {
	ds, err := datadir.Load(filepath.Join("..", "..", "examples", "data"))
	if err != nil {
		t.Fatalf("load data dir: %v", err)
	}

	for _, nodeID := range []string{"", "..", "../../escaped", "a/b"} {
		t.Run(nodeID, func(t *testing.T) {
			parent := t.TempDir()
			stateDir := filepath.Join(parent, "state")
			if err := os.Mkdir(stateDir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			srv := &server{ds: ds, stateDir: stateDir}

			body, err := json.Marshal(data.Spec{
				Machine: "qemu_vm",
				OS:      "debian13",
				NodeID:  nodeID,
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/nodes", strings.NewReader(string(body)))
			rec := httptest.NewRecorder()
			srv.handleCreateNode(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status: got %d, want 400", rec.Code)
			}
			assertOnlyEntry(t, parent, "state")
			assertEmptyDir(t, stateDir)
		})
	}
}

// TestHandleGetConfig_DeletedNode verifies that a node's artifacts
// are served while it is registered, are not served after DELETE, and
// stay on disk.
func TestHandleGetConfig_DeletedNode(t *testing.T) {
	ds, err := datadir.Load(filepath.Join("..", "..", "examples", "data"))
	if err != nil {
		t.Fatalf("load data dir: %v", err)
	}
	stateDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(stateDir, "nodes"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srv := &server{ds: ds, stateDir: stateDir}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /nodes", srv.handleCreateNode)
	mux.HandleFunc("DELETE /nodes/{node_id}", srv.handleDeleteNode)
	mux.HandleFunc("GET /configs/{node_id}/{file...}", srv.handleGetConfig)

	do := func(method, path, body string) int {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec.Code
	}

	spec := `{"machine":"qemu_vm","os":"debian13","node_id":"node1"}`
	if got := do(http.MethodPost, "/nodes", spec); got != http.StatusOK {
		t.Fatalf("POST: got %d, want 200", got)
	}
	if got := do(http.MethodGet, "/configs/node1/boot.ipxe", ""); got != http.StatusOK {
		t.Errorf("GET while registered: got %d, want 200", got)
	}
	if got := do(http.MethodDelete, "/nodes/node1", ""); got != http.StatusNoContent {
		t.Fatalf("DELETE: got %d, want 204", got)
	}
	if got := do(http.MethodGet, "/configs/node1/boot.ipxe", ""); got != http.StatusNotFound {
		t.Errorf("GET after DELETE: got %d, want 404", got)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "configs", "node1", "boot.ipxe")); err != nil {
		t.Errorf("artifact after DELETE: %v", err)
	}
}

func assertOnlyEntry(t *testing.T, dir, want string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != want {
		t.Errorf("%s: got entries %v, want only %q", dir, entries, want)
	}
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("%s: got entries %v, want none", dir, entries)
	}
}

// TestHandleGetConfig_PathTraversal verifies that handleGetConfig
// refuses every form of path traversal we have considered. The
// handler is the only place that turns an untrusted URL component
// into a filesystem path, so a regression here is a security bug.
//
// Some traversal forms are rejected by Go's http.ServeMux before the
// handler is reached (it canonicalizes "." segments and "/..//" forms
// in the URL path). Tests below check the end-to-end property -- the
// secret outside configs/ must never be returned -- regardless of
// whether the rejection happens in the mux or in the handler.
func TestHandleGetConfig_PathTraversal(t *testing.T) {
	stateDir := t.TempDir()

	// Place a real artifact and a "secret" outside configs/ to make
	// sure traversal attempts cannot reach the secret.
	nodeDir := filepath.Join(stateDir, "configs", "node1", "post")
	if err := os.MkdirAll(nodeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "configs", "node1", "boot.ipxe"),
		[]byte("ipxe-content"), 0o644); err != nil {
		t.Fatalf("write boot.ipxe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeDir, "udev.sh"),
		[]byte("udev-content"), 0o755); err != nil {
		t.Fatalf("write udev.sh: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(stateDir, "nodes"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "nodes", "node1.json"),
		[]byte("{}"), 0o644); err != nil {
		t.Fatalf("write node1.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "secret.txt"),
		[]byte("SECRET"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	srv := &server{stateDir: stateDir}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /configs/{node_id}/{file...}", srv.handleGetConfig)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	tests := []struct {
		name     string
		path     string
		wantOK   bool
		wantBody string // body to check (always checked against secret leak)
	}{
		{
			name:     "top-level file",
			path:     "/configs/node1/boot.ipxe",
			wantOK:   true,
			wantBody: "ipxe-content",
		},
		{
			name:     "subpath file",
			path:     "/configs/node1/post/udev.sh",
			wantOK:   true,
			wantBody: "udev-content",
		},
		{
			name:   "missing file is not served as secret",
			path:   "/configs/node1/does-not-exist",
			wantOK: false,
		},
		{
			name:   "dotdot in file does not reach the secret",
			path:   "/configs/node1/post/../../secret.txt",
			wantOK: false,
		},
		{
			name:   "url-encoded dotdot does not reach the secret",
			path:   "/configs/node1/%2e%2e/secret.txt",
			wantOK: false,
		},
		{
			name:   "double-url-encoded dotdot does not reach the secret",
			path:   "/configs/node1/%252e%252e/secret.txt",
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ts.Client().Get(ts.URL + tc.path)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			defer resp.Body.Close()
			buf := make([]byte, 1024)
			n, _ := resp.Body.Read(buf)
			body := string(buf[:n])

			// In every case, the response must not contain the secret.
			if body == "SECRET" {
				t.Fatalf("traversal succeeded: body is the secret (status=%d)", resp.StatusCode)
			}

			if tc.wantOK {
				if resp.StatusCode != http.StatusOK {
					t.Errorf("status: got %d, want 200", resp.StatusCode)
				}
				if body != tc.wantBody {
					t.Errorf("body: got %q, want %q", body, tc.wantBody)
				}
				return
			}

			if resp.StatusCode == http.StatusOK {
				t.Errorf("got 200 OK for forbidden path; body=%q", body)
			}
		})
	}
}
