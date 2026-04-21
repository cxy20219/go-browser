package commands

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeTargetURL(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(filePath, []byte("<h1>hello</h1>"), 0644); err != nil {
		t.Fatal(err)
	}

	abs, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatal(err)
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "full url", input: "https://example.com/path", want: "https://example.com/path"},
		{name: "bare domain", input: "example.com", want: "https://example.com"},
		{name: "local file", input: filePath, want: fileURL},
		{name: "selector-like input", input: "#login", want: "#login"},
		{name: "localhost path", input: "localhost:3000/app", want: "localhost:3000/app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeTargetURL(tt.input)
			if err != nil {
				t.Fatalf("normalizeTargetURL(%q) returned error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("normalizeTargetURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseViewportSize(t *testing.T) {
	width, height, err := parseViewportSize("1280", "720")
	if err != nil {
		t.Fatalf("parseViewportSize returned error: %v", err)
	}
	if width != 1280 || height != 720 {
		t.Fatalf("parseViewportSize = %dx%d, want 1280x720", width, height)
	}

	if _, _, err := parseViewportSize("wide", "720"); err == nil {
		t.Fatalf("expected invalid width to return an error")
	}
	if _, _, err := parseViewportSize("1280", "tall"); err == nil {
		t.Fatalf("expected invalid height to return an error")
	}
}
