package plan

import (
	"strings"
	"testing"
)

func TestNpmCIFallbackNote(t *testing.T) {
	note := npmCIFallbackNote()

	if strings.Count(note, ".") != 2 {
		t.Fatalf("expected two sentences, got %q", note)
	}
	if strings.Contains(note, "\n") {
		t.Fatalf("expected a single-line message, got %q", note)
	}
	for _, want := range []string{
		"npm ci failed",
		"npm install instead",
		"lockfile may be out of sync",
		"Commit the updated lockfile",
		"report the issue upstream",
	} {
		if !strings.Contains(note, want) {
			t.Fatalf("expected note to contain %q, got %q", want, note)
		}
	}
}
