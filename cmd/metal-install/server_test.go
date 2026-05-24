package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

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
