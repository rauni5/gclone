package explain

import (
	"errors"
	"testing"

	"github.com/rauni5/gclone/internal/proc"
)

func TestRules(t *testing.T) {
	cases := []struct{ name, out, id string }{
		{"eresolve", "npm ERR! code ERESOLVE\nnpm ERR! ERESOLVE could not resolve", "npm-eresolve"},
		{"etarget", "npm error code ETARGET\nnpm error notarget No matching version", "npm-target"},
		{"lifecycle", "npm ERR! code ELIFECYCLE", "npm-lifecycle"},
		{"gyp", "gyp ERR! find Python", "node-gyp"},
		{"port", "Error: listen EADDRINUSE: address already in use :::3000", "port-in-use"},
		{"spring port", "Web server failed to start. Port 8080 was already in use.", "port-in-use"},
		{"maven offline", "Could not transfer artifact x from central: Unknown host repo.maven.apache.org", "network"},
		{"npm dns", "npm ERR! code ENOTFOUND", "network"},
		{"java old", "error: release version 17 not supported", "java-too-old"},
		{"java home", "The JAVA_HOME environment variable is not defined correctly", "java-home"},
		{"go old", "go: github.com/x/y requires go >= 1.24 (running go 1.21)", "go-version"},
		{"auth", "npm ERR! code E401", "auth"},
		{"git", "fatal: Repository not found.", "git-access"},
		{"docker", "pull access denied for example/app", "docker-pull"},
		{"tls", "SELF_SIGNED_CERT_IN_CHAIN", "tls-intercept"},
		{"disk", "ENOSPC: no space left on device", "disk"},
	}
	for _, c := range cases {
		h, ok := For(c.out)
		if !ok || h.ID != c.id {
			t.Errorf("%s: got %+v ok=%v, want %s", c.name, h, ok, c.id)
		}
	}
	if _, ok := For("everything went fine, nothing to see"); ok {
		t.Error("unrelated output should give no hint")
	}
}

func TestFromError(t *testing.T) {
	err := error(&proc.RunError{Err: errors.New("exit status 1"), Output: "npm ERR! ERESOLVE"})
	wrapped := errors.Join(errors.New("setup"), err)
	if h, ok := FromError(wrapped); !ok || h.ID != "npm-eresolve" {
		t.Errorf("got %+v %v", h, ok)
	}
	if _, ok := FromError(errors.New("plain")); ok {
		t.Error("a plain error has no output to explain")
	}
}

func TestRunStreamCapturesOutput(t *testing.T) {
	err := proc.RunStream("", false, "sh", "-c", "echo npm ERR! ERESOLVE >&2; exit 3")
	h, ok := FromError(err)
	if !ok || h.ID != "npm-eresolve" {
		t.Fatalf("got %+v %v (err=%v)", h, ok, err)
	}
}
