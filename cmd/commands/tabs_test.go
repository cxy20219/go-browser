package commands

import "testing"

func TestParseTabIndex(t *testing.T) {
	index, err := parseTabIndex("2")
	if err != nil {
		t.Fatalf("parseTabIndex returned error: %v", err)
	}
	if index != 2 {
		t.Fatalf("parseTabIndex = %d, want 2", index)
	}

	if _, err := parseTabIndex("two"); err == nil {
		t.Fatalf("expected invalid tab index to return an error")
	}
}
