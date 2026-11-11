package golang

import "testing"

func TestParseGoMod(t *testing.T) {
	mod, v, tc := ParseGoMod("module github.com/rauni5/gclone\n\ngo 1.24\ntoolchain go1.24.2\n")
	if mod != "github.com/rauni5/gclone" || v != "1.24" || tc != "1.24.2" {
		t.Fatalf("got %q %q %q", mod, v, tc)
	}
	mod, v, tc = ParseGoMod("module example.com/x\ngo 1.21.5\n")
	if mod != "example.com/x" || v != "1.21.5" || tc != "" {
		t.Fatalf("got %q %q %q", mod, v, tc)
	}
}

func TestOlder(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.21", "1.24", true}, {"1.24.0", "1.24", false}, {"1.24.1", "1.24.2", true},
		{"1.25", "1.24.9", false}, {"", "1.24", true},
	}
	for _, c := range cases {
		if got := Older(c.a, c.b); got != c.want {
			t.Errorf("Older(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}
