package webui_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/sigxcpu/tv-dvr208/backend/internal/webui"
)

func TestEmbeddedDistFS(t *testing.T) {
	distFS, err := webui.DistFS()
	if err != nil {
		t.Fatalf("DistFS() error: %v", err)
	}

	indexBytes, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		t.Fatalf("failed to read embedded index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), `<div id="root"></div>`) {
		t.Fatalf("unexpected embedded index.html content: %s", string(indexBytes))
	}

	entries, err := fs.ReadDir(distFS, "assets")
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected embedded assets/ directory with bundled JS/CSS, got len=%d (err=%v)", len(entries), err)
	}
}
