package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestFitToHeight(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	lines[25] = focusMarker + "focused"
	lines = append(lines, "help 1", "help 2")
	view := strings.Join(lines, "\n")

	got := strings.Split(fitToHeight(view, 10, 2), "\n")
	if len(got) != 10 {
		t.Fatalf("got %d lines, want 10", len(got))
	}
	if got[8] != "help 1" || got[9] != "help 2" {
		t.Errorf("footer not kept: %q", got[8:])
	}
	if !strings.Contains(strings.Join(got, "\n"), "focused") {
		t.Errorf("focused field scrolled out: %q", got)
	}
	if fitToHeight(view, 100, 2) != view {
		t.Error("a view that fits must not change")
	}
}
