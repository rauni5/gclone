package verify

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/analyze"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
)

func TestPortFromLog(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"  Local:   http://localhost:5173/", 5173},
		{"\x1b[32m  ➜  Local:\x1b[0m   http://localhost:\x1b[1m5174\x1b[22m/", 5174},
		{"Server listening on port 4000", 4000},
		{"ready - started server on 0.0.0.0:3000, url: http://localhost:3000", 3000},
		{"connected to postgres://u:p@localhost:5432/app", 0}, // database URL, not the app
		{"running on [::1]:8080", 8080},
		{"compiled successfully", 0},
	}
	for _, c := range cases {
		if got := portFromLog(c.in); got != c.want {
			t.Errorf("portFromLog(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestTailBuf(t *testing.T) {
	tb := newTail()
	tb.max = 40
	fmt.Fprint(tb, "one\ntwo\n\x1b[31mthree\x1b[0m\nfour\n")
	got := tb.Tail(2)
	if got != "three\nfour" {
		t.Errorf("tail = %q", got)
	}
	fmt.Fprint(tb, strings.Repeat("x", 100))
	if len(tb.buf) > 40 {
		t.Errorf("buffer should be capped, has %d bytes", len(tb.buf))
	}
}

func TestVerdict(t *testing.T) {
	r := &project.Report{}
	if v := verdictOf(r); v != "READY" {
		t.Errorf("empty report = %s", v)
	}
	r.Units = []project.UnitReport{{Checks: []project.Check{{Name: "env", Status: "warn", Detail: "x"}}}}
	if v := verdictOf(r); v != "READY WITH WARNINGS" {
		t.Errorf("warning report = %s", v)
	}
	r.Services = []project.Check{{Name: "PostgreSQL", Status: "fail", Detail: "x"}}
	if v := verdictOf(r); v != "NOT READY" {
		t.Errorf("failing report = %s", v)
	}
}

func TestEnvCheck(t *testing.T) {
	if c := envCheck(project.Runnable{ExampleName: ".env.example"}); c.Status != "fail" {
		t.Errorf("missing .env should fail: %+v", c)
	}
	if c := envCheck(project.Runnable{ExampleName: ".env.example", HasEnv: true, Blocking: []string{"KEY"}}); c.Status != "warn" {
		t.Errorf("placeholder should warn: %+v", c)
	}
	if c := envCheck(project.Runnable{ExampleName: ".env.example", HasEnv: true}); c.Status != "pass" {
		t.Errorf("clean env should pass: %+v", c)
	}
	if c := envCheck(project.Runnable{}); c.Status != "skip" {
		t.Errorf("no env at all should skip: %+v", c)
	}
}

func TestHTTPProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	status, err := httpProbe(n)
	if err != nil || status != http.StatusFound {
		t.Fatalf("probe = %d, %v (redirects must not be followed)", status, err)
	}
}

func TestUpdateGuideVerification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, project.GuideName)
	_ = os.WriteFile(path, []byte("# Guide\n\nbody\n"), 0o644)
	r := &project.Report{When: time.Now(), Verdict: "READY"}
	updateGuideVerification(dir, r)
	r2 := &project.Report{When: time.Now(), Verdict: "NOT READY", Services: []project.Check{{Name: "Redis", Status: "fail", Detail: "down"}}}
	updateGuideVerification(dir, r2)
	b, _ := os.ReadFile(path)
	s := string(b)
	if strings.Count(s, verifyStart) != 1 || !strings.Contains(s, "NOT READY") || strings.Contains(s, "Result: **READY**") {
		t.Errorf("section should be replaced, not duplicated:\n%s", s)
	}
	if !strings.Contains(s, "# Guide") {
		t.Error("existing guide content was lost")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func needNode(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not installed")
	}
	if node.InstalledNode().Major < 20 {
		t.Skip("needs Node 20 or newer for --env-file")
	}
}

func nodeProject(t *testing.T, script, serverJS string, port int) string {
	t.Helper()
	dir := t.TempDir()
	pj := fmt.Sprintf(`{"name":"vt","scripts":{"dev":%q}}`, script)
	files := map[string]string{
		"package.json": pj,
		"server.js":    serverJS,
		".env":         fmt.Sprintf("PORT=%d\n", port),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func findCheck(cs []project.Check, name string) project.Check {
	for _, c := range cs {
		if c.Name == name {
			return c
		}
	}
	return project.Check{Name: name, Status: "missing"}
}

func quickOpts() VerifyOptions {
	return VerifyOptions{startTimeout: 20 * time.Second, buildTimeout: time.Minute}
}

func TestVerifyWorkingServer(t *testing.T) {
	needNode(t)
	port := freePort(t)
	dir := nodeProject(t, "node --env-file=.env server.js",
		"require('http').createServer((q,r)=>r.end('ok')).listen(process.env.PORT)", port)
	vo := quickOpts()
	rs := node.Runnables(analyze.AnalyzeProject(dir), vo.NoBuild)
	checks := verifyRunnable(context.Background(), rs[0], vo)
	if c := findCheck(checks, "start"); c.Status != "pass" {
		t.Fatalf("start should pass: %+v", c)
	}
	if c := findCheck(checks, "http"); c.Status != "pass" || !strings.Contains(c.Detail, "200") {
		t.Errorf("http should pass with 200: %+v", c)
	}
	time.Sleep(300 * time.Millisecond)
	if services.Reachable(port) {
		t.Error("the server should have been stopped after verification")
	}
}

func TestVerifyCrashingServer(t *testing.T) {
	needNode(t)
	port := freePort(t)
	dir := nodeProject(t, "node server.js", "console.error('boom: cannot connect'); process.exit(3)", port)
	vo := quickOpts()
	rs := node.Runnables(analyze.AnalyzeProject(dir), vo.NoBuild)
	c := findCheck(verifyRunnable(context.Background(), rs[0], vo), "start")
	if c.Status != "fail" || !strings.Contains(c.Detail, "boom: cannot connect") {
		t.Fatalf("a crash should fail and show the log: %+v", c)
	}
}

func TestVerifyPortInUse(t *testing.T) {
	needNode(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	dir := nodeProject(t, "node server.js", "setInterval(()=>{},1000)", port)
	vo := quickOpts()
	rs := node.Runnables(analyze.AnalyzeProject(dir), vo.NoBuild)
	c := findCheck(verifyRunnable(context.Background(), rs[0], vo), "start")
	if c.Status != "fail" || !strings.Contains(c.Detail, "already in use") {
		t.Fatalf("busy port should fail early: %+v", c)
	}
}

func TestVerifyTimeoutWithoutPort(t *testing.T) {
	needNode(t)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"q","scripts":{"dev":"node server.js"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "server.js"), []byte("setInterval(()=>{},1000)"), 0o644)
	vo := VerifyOptions{startTimeout: 3 * time.Second, buildTimeout: time.Minute}
	rs := node.Runnables(analyze.AnalyzeProject(dir), vo.NoBuild)
	c := findCheck(verifyRunnable(context.Background(), rs[0], vo), "start")
	if c.Status != "warn" || !strings.Contains(c.Detail, "no port could be determined") {
		t.Fatalf("a silent long-running app should warn, not pass: %+v", c)
	}
}

func TestVerifyBuildFailureSkipsStart(t *testing.T) {
	needNode(t)
	dir := t.TempDir()
	pj := `{"name":"b","scripts":{"build":"node build.js","dev":"node server.js"}}`
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(pj), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "build.js"), []byte("console.error('TS2304: cannot find name'); process.exit(2)"), 0o644)
	vo := quickOpts()
	rs := node.Runnables(analyze.AnalyzeProject(dir), vo.NoBuild)
	checks := verifyRunnable(context.Background(), rs[0], vo)
	if c := findCheck(checks, "build"); c.Status != "fail" || !strings.Contains(c.Detail, "TS2304") {
		t.Errorf("build failure should show the log: %+v", c)
	}
	if c := findCheck(checks, "start"); c.Status != "skip" {
		t.Errorf("start should be skipped after a failed build: %+v", c)
	}
}
