package snapshot

import (
	"strings"
	"testing"
	"time"
)

func TestRefCacheBuildFromSnapshotClearsOldRefs(t *testing.T) {
	cache := NewRefCache()
	cache.Set("e99", ElementRef{Ref: "e99", Selector: `[data-go-browser-ref="e99"]`})

	cache.BuildFromSnapshot(&Snapshot{
		Elements: []ElementRef{
			{Ref: "e0", Selector: `[data-go-browser-ref="e0"]`},
			{Ref: "e1", Selector: `[data-go-browser-ref="e1"]`},
		},
	})

	if _, ok := cache.Get("e99"); ok {
		t.Fatalf("expected stale ref e99 to be cleared")
	}
	if selector, ok := cache.Selector("e1"); !ok || selector != `[data-go-browser-ref="e1"]` {
		t.Fatalf("expected cached selector for e1, got %q ok=%v", selector, ok)
	}
}

func TestIsRef(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"e0", true},
		{" e12 ", true},
		{"eabc", true},
		{"e12345", false},
		{"#e0", false},
		{"button", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsRef(tt.value); got != tt.want {
			t.Fatalf("IsRef(%q) = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestFormatSnapshotHidesInternalSelector(t *testing.T) {
	text := FormatSnapshot(&Snapshot{
		URL:       "https://example.com",
		Title:     "Example",
		Timestamp: time.Unix(0, 0),
		Elements: []ElementRef{
			{
				Ref:      "e0",
				Tag:      "button",
				Text:     "Click",
				Selector: `[data-go-browser-ref="e0"]`,
				Visible:  true,
			},
		},
	})

	if strings.Contains(text, "data-go-browser-ref") {
		t.Fatalf("internal selector leaked into formatted snapshot: %s", text)
	}
	if !strings.Contains(text, `[e0] button "Click"`) {
		t.Fatalf("formatted snapshot missing element ref/text: %s", text)
	}
}
