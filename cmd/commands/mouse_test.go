package commands

import "testing"

func TestParseMousePair(t *testing.T) {
	x, y, err := parseMousePair("12.5", "-3")
	if err != nil {
		t.Fatalf("parseMousePair returned error: %v", err)
	}
	if x != 12.5 || y != -3 {
		t.Fatalf("parseMousePair = %v, %v, want 12.5, -3", x, y)
	}

	if _, _, err := parseMousePair("left", "10"); err == nil {
		t.Fatalf("expected invalid first value to return an error")
	}
	if _, _, err := parseMousePair("10", "down"); err == nil {
		t.Fatalf("expected invalid second value to return an error")
	}
}

func TestLocalMouseButtonOptions(t *testing.T) {
	if _, err := localMouseDownOptions(""); err != nil {
		t.Fatalf("left/default mouse down should be valid: %v", err)
	}
	if _, err := localMouseDownOptions("right"); err != nil {
		t.Fatalf("right mouse down should be valid: %v", err)
	}
	if _, err := localMouseDownOptions("middle"); err != nil {
		t.Fatalf("middle mouse down should be valid: %v", err)
	}
	if _, err := localMouseUpOptions("middle"); err != nil {
		t.Fatalf("middle mouse up should be valid: %v", err)
	}
	if _, err := localMouseDownOptions("side"); err == nil {
		t.Fatalf("unsupported mouse button should return an error")
	}
}
