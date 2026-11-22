package engine

// Multipart file upload tests (P0 task 7.2): file items (KV.Type == "file")
// carry a filesystem path in Value; the engine reads the file at send time
// and sends it via CreateFormFile, next to plain text fields.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestRunMultipartFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report data.txt")
	payload := []byte("file-bytes-123")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	var (
		gotText     string
		gotFileName string
		gotContent  []byte
		gotCT       string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("server parse: %v", err)
			return
		}
		gotText = r.FormValue("note")
		f, fh, err := r.FormFile("upload")
		if err != nil {
			t.Errorf("server FormFile: %v", err)
			return
		}
		defer f.Close()
		gotFileName = fh.Filename
		gotContent, _ = io.ReadAll(f)
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body: &model.Body{
			Type: model.BodyMultipart,
			FormData: []model.KV{
				{Name: "note", Value: "hello"},
				{Name: "upload", Type: "file", Value: path, FileName: "display-name.txt"},
			},
		},
	}, nil, "")
	if res.Error != "" {
		t.Fatalf("error: %s", res.Error)
	}
	if res.Status != 200 {
		t.Fatalf("status = %d", res.Status)
	}
	if gotText != "hello" {
		t.Fatalf("server text field = %q", gotText)
	}
	if !strings.Contains(gotCT, "multipart/form-data") {
		t.Fatalf("content type = %q", gotCT)
	}
	// The sent filename is the base name of the stored path.
	if gotFileName != filepath.Base(path) {
		t.Fatalf("server filename = %q, want %q", gotFileName, filepath.Base(path))
	}
	if !bytes.Equal(gotContent, payload) {
		t.Fatalf("server content = %q, want %q", gotContent, payload)
	}
}

func TestRunMultipartFileMissing(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	exec := &Executor{}
	res := exec.Run(&model.HttpRequest{
		Method: http.MethodPost,
		URL:    srv.URL,
		Body: &model.Body{
			Type: model.BodyMultipart,
			FormData: []model.KV{
				{Name: "upload", Type: "file", Value: "/nonexistent/dir/file.bin"},
			},
		},
	}, nil, "")
	// Same error style as the other build-time failures: Run records the
	// error and nothing is sent.
	if res.Error == "" {
		t.Fatal("expected error for missing file, got none")
	}
	if res.Status != 0 {
		t.Fatalf("status = %d, want 0", res.Status)
	}
	if called {
		t.Fatal("request was sent despite unreadable file")
	}
}

func stringsContains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
