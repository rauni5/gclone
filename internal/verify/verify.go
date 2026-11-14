package verify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rauni5/gclone/internal/adapter/golang"
	"github.com/rauni5/gclone/internal/adapter/java"
	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/analyze"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
)

var errNotReady = errors.New("verification failed: the project is not ready")

const verifyStart = "<!-- gclone:verify:start -->"

const verifyEnd = "<!-- gclone:verify:end -->"

type VerifyOptions struct {
	startTimeout time.Duration
	buildTimeout time.Duration
	NoBuild      bool
}

func verifyOptionsFrom(o project.Options) VerifyOptions {
	t := time.Duration(o.Timeout) * time.Second
	if t <= 0 {
		t = 60 * time.Second
	}
	return VerifyOptions{startTimeout: t, buildTimeout: 5 * time.Minute, NoBuild: o.NoBuild}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

var logPortRe = regexp.MustCompile(`(?i)(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1?\]|::):(\d{2,5})|\bport\b[^\d\n]{0,6}(\d{2,5})`)

var logHintRe = regexp.MustCompile(`(?i)(listen|running|started|ready|local:|server|http)`)

var logDBRe = regexp.MustCompile(`(?i)(postgres(ql)?|mongodb(\+srv)?|mysql|redis|amqp)://`)

// portFromLog finds the port a dev server announces in its output.
func portFromLog(s string) int {
	for _, line := range strings.Split(ansiRe.ReplaceAllString(s, ""), "\n") {
		if !logHintRe.MatchString(line) || logDBRe.MatchString(line) {
			continue
		}
		for _, m := range logPortRe.FindAllStringSubmatch(line, -1) {
			for _, g := range m[1:] {
				if n, err := strconv.Atoi(g); err == nil && n >= 1 && n <= 65535 {
					return n
				}
			}
		}
	}
	return 0
}

// tailBuf keeps the end of a program's output and notices the port it announces.
type tailBuf struct {
	mu   sync.Mutex
	buf  []byte
	max  int
	port int
}

func newTail() *tailBuf { return &tailBuf{max: 16 << 10} }

func (t *tailBuf) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
	if t.port == 0 {
		t.port = portFromLog(string(p))
	}
	return len(p), nil
}

func (t *tailBuf) Port() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.port
}

// Tail returns the last n non-empty lines without colour codes.
func (t *tailBuf) Tail(n int) string {
	t.mu.Lock()
	s := string(t.buf)
	t.mu.Unlock()
	var lines []string
	for _, l := range strings.Split(ansiRe.ReplaceAllString(s, ""), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func startProc(dir string, args []string, out io.Writer) (*exec.Cmd, chan error, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = out, out
	// Keep dev servers from opening a browser or printing colour codes.
	cmd.Env = append(os.Environ(), "BROWSER=none", "NO_COLOR=1", "FORCE_COLOR=0")
	proc.SetProcGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return cmd, done, nil
}

// stopProc stops the process tree, asking first and forcing after a few seconds.
func stopProc(cmd *exec.Cmd, done chan error) {
	proc.SignalGroup(cmd, false)
	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
	}
	proc.SignalGroup(cmd, true)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

func since(t time.Time) string {
	return time.Since(t).Round(100 * time.Millisecond).String()
}

func envCheck(r project.Runnable) project.Check {
	var parts []string
	if len(r.Blocking) > 0 {
		parts = append(parts, "still empty or a placeholder: "+strings.Join(r.Blocking, ", "))
	}
	if len(r.Missing) > 0 {
		parts = append(parts, "read by the code but not defined: "+strings.Join(r.Missing, ", "))
	}
	switch {
	case r.ExampleName != "" && !r.HasEnv:
		return project.Check{Name: "env", Status: "fail", Detail: "no .env file. Copy " + r.ExampleName + " to .env and fill it in"}
	case len(parts) > 0:
		return project.Check{Name: "env", Status: "warn", Detail: strings.Join(parts, "; ")}
	case r.ExampleName == "" && !r.HasEnv:
		return project.Check{Name: "env", Status: "skip", Detail: "no env file is used"}
	}
	return project.Check{Name: "env", Status: "pass", Detail: ".env present, no empty or placeholder values"}
}

func buildCheck(ctx context.Context, r project.Runnable, vo VerifyOptions) project.Check {
	label := strings.Join(r.Build, " ")
	tb := newTail()
	began := time.Now()
	cmd, done, err := startProc(r.Dir, r.Build, tb)
	if err != nil {
		return project.Check{Name: "build", Status: "fail", Detail: fmt.Sprintf("could not run %s: %v", label, err)}
	}
	timer := time.NewTimer(vo.buildTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			return project.Check{Name: "build", Status: "fail", Detail: fmt.Sprintf("%s failed: %v\n%s", label, err, tb.Tail(12))}
		}
		return project.Check{Name: "build", Status: "pass", Detail: fmt.Sprintf("%s succeeded in %s", label, since(began))}
	case <-ctx.Done():
		stopProc(cmd, done)
		return project.Check{Name: "build", Status: "skip", Detail: "interrupted"}
	case <-timer.C:
		stopProc(cmd, done)
		return project.Check{Name: "build", Status: "fail", Detail: fmt.Sprintf("%s did not finish within %s\n%s", label, vo.buildTimeout, tb.Tail(8))}
	}
}

func httpProbe(port int) (int, error) {
	client := &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	var lastErr error
	for _, host := range []string{"127.0.0.1", "[::1]"} {
		resp, err := client.Get(fmt.Sprintf("http://%s:%d/", host, port))
		if err == nil {
			resp.Body.Close()
			return resp.StatusCode, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

func effectivePort(tb *tailBuf, r project.Runnable) int {
	if p := tb.Port(); p > 0 {
		return p
	}
	return r.Port
}

func startChecks(ctx context.Context, r project.Runnable, vo VerifyOptions) []project.Check {
	if len(r.Start) == 0 && r.NoStart != "" {
		return []project.Check{{Name: "start", Status: "skip", Detail: r.NoStart}}
	}
	if len(r.Start) == 0 {
		return []project.Check{{Name: "start", Status: "warn", Detail: "no dev, start, serve or develop script in package.json, so it was not started"}}
	}
	label := strings.Join(r.Start, " ")
	if r.Port > 0 && services.Reachable(r.Port) {
		return []project.Check{{Name: "start", Status: "fail", Detail: fmt.Sprintf(
			"port %d (%s) is already in use by another program, so the result could not be trusted. Stop it or change PORT, then verify again", r.Port, r.PortFrom)}}
	}

	tb := newTail()
	began := time.Now()
	cmd, done, err := startProc(r.Dir, r.Start, tb)
	if err != nil {
		return []project.Check{{Name: "start", Status: "fail", Detail: fmt.Sprintf("could not run %s: %v", label, err)}}
	}
	exited := false
	defer func() {
		if !exited {
			stopProc(cmd, done)
		}
	}()

	deadline := time.NewTimer(vo.startTimeout)
	defer deadline.Stop()
	tick := time.NewTicker(400 * time.Millisecond)
	defer tick.Stop()

	port := 0
	for port == 0 {
		select {
		case err := <-done:
			exited = true
			status := "exit status 0"
			if err != nil {
				status = err.Error()
			}
			return []project.Check{{Name: "start", Status: "fail", Detail: fmt.Sprintf("%s stopped before it started listening (%s)\n%s", label, status, tb.Tail(12))}}
		case <-ctx.Done():
			return []project.Check{{Name: "start", Status: "skip", Detail: "interrupted"}}
		case <-deadline.C:
			if eff := effectivePort(tb, r); eff > 0 {
				return []project.Check{{Name: "start", Status: "fail", Detail: fmt.Sprintf("%s did not listen on port %d within %s\n%s", label, eff, vo.startTimeout, tb.Tail(12))}}
			}
			return []project.Check{{Name: "start", Status: "warn", Detail: fmt.Sprintf(
				"%s was still running after %s, but no port could be determined, so it is not confirmed to work\n%s", label, vo.startTimeout, tb.Tail(6))}}
		case <-tick.C:
			if eff := effectivePort(tb, r); eff > 0 && services.Reachable(eff) {
				port = eff
			}
		}
	}

	checks := []project.Check{{Name: "start", Status: "pass", Detail: fmt.Sprintf("%s is listening on port %d after %s", label, port, since(began))}}
	status, err := httpProbe(port)
	switch {
	case err != nil:
		checks = append(checks, project.Check{Name: "http", Status: "warn", Detail: fmt.Sprintf("port %d is open but did not answer HTTP: %v", port, err)})
	case status >= 500:
		checks = append(checks, project.Check{Name: "http", Status: "warn", Detail: fmt.Sprintf("GET / answered %d", status)})
	default:
		checks = append(checks, project.Check{Name: "http", Status: "pass", Detail: fmt.Sprintf("GET / answered %d", status)})
	}
	return checks
}

func verifyRunnable(ctx context.Context, r project.Runnable, vo VerifyOptions) []project.Check {
	checks := []project.Check{envCheck(r)}
	if len(r.Build) > 0 {
		c := buildCheck(ctx, r, vo)
		checks = append(checks, c)
		switch {
		case c.Status == "fail":
			return append(checks, project.Check{Name: "start", Status: "skip", Detail: "not tried because the build failed"})
		case ctx.Err() != nil:
			return checks
		}
	}
	return append(checks, startChecks(ctx, r, vo)...)
}

func verifyServices(p *project.Project) []project.Check {
	var out []project.Check
	for _, n := range p.Needs {
		switch {
		case n.Remote:
			out = append(out, project.Check{Name: n.Name, Status: "skip", Detail: "uses remote host " + n.RemoteHost + ", not checked"})
		case services.Reachable(n.Port):
			out = append(out, project.Check{Name: n.Name, Status: "pass", Detail: fmt.Sprintf("accepting connections on localhost:%d", n.Port)})
		default:
			out = append(out, project.Check{Name: n.Name, Status: "fail", Detail: fmt.Sprintf("nothing is listening on localhost:%d (needed by %s)", n.Port, strings.Join(n.Reasons, ", "))})
		}
	}
	return out
}

func verdictOf(r *project.Report) string {
	f, w := r.Counts()
	switch {
	case f > 0:
		return "NOT READY"
	case w > 0:
		return "READY WITH WARNINGS"
	}
	return "READY"
}

var statusTag = map[string]string{"pass": "[ok]  ", "warn": "[warn]", "fail": "[FAIL]", "skip": "[skip]"}

func printCheck(c project.Check) {
	lines := strings.Split(c.Detail, "\n")
	fmt.Printf("    %s %s: %s\n", statusTag[c.Status], c.Name, lines[0])
	for _, l := range lines[1:] {
		fmt.Printf("             | %s\n", l)
	}
}

func UnitLabel(l string) string {
	if l == "" || l == "." {
		return "repo root"
	}
	return l
}

// runVerify checks services, then builds, starts and probes each part in turn.
func RunVerify(p *project.Project, o project.Options) *project.Report {
	vo := verifyOptionsFrom(o)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rep := &project.Report{Dir: p.Dir, When: time.Now()}
	fmt.Println("\nVerifying. Each part is built, started for a few seconds, probed and stopped.")

	if rep.Services = verifyServices(p); len(rep.Services) > 0 {
		fmt.Println("\n  Services")
		for _, c := range rep.Services {
			printCheck(c)
		}
	}
	runnables := append(node.Runnables(p, vo.NoBuild), golang.Runnables(p, vo.NoBuild)...)
	runnables = append(runnables, java.Runnables(p, vo.NoBuild)...)
	for _, r := range runnables {
		if ctx.Err() != nil {
			break
		}
		fmt.Printf("\n  %s\n", UnitLabel(r.Label))
		u := project.UnitReport{Label: r.Label, Checks: verifyRunnable(ctx, r, vo)}
		for _, c := range u.Checks {
			printCheck(c)
		}
		rep.Units = append(rep.Units, u)
	}
	rep.Verdict = verdictOf(rep)
	if ctx.Err() != nil {
		rep.Verdict = "INTERRUPTED"
	}
	fmt.Printf("\nResult: %s\n", rep.Summary())
	return rep
}

func VerifySection(r *project.Report) string {
	var b strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&b, format+"\n", a...) }
	item := func(c project.Check) {
		lines := strings.Split(c.Detail, "\n")
		w("- **%s** %s: %s", c.Status, c.Name, lines[0])
		if len(lines) > 1 {
			w("")
			w("  ```")
			for _, l := range lines[1:] {
				w("  %s", l)
			}
			w("  ```")
		}
	}
	w("%s", verifyStart)
	w("## Verification")
	w("")
	w("Result: **%s**, checked %s.", r.Summary(), r.When.Format("2006-01-02 15:04"))
	w("")
	if len(r.Services) > 0 {
		w("### Services")
		w("")
		for _, c := range r.Services {
			item(c)
		}
		w("")
	}
	for _, u := range r.Units {
		w("### %s", UnitLabel(u.Label))
		w("")
		for _, c := range u.Checks {
			item(c)
		}
		w("")
	}
	w("Run `gclone verify` again after changing anything.")
	w("%s", verifyEnd)
	w("")
	return b.String()
}

// updateGuideVerification replaces the verification section of an existing guide.
func updateGuideVerification(dir string, r *project.Report) {
	path := filepath.Join(dir, project.GuideName)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text, section := string(b), VerifySection(r)
	start, end := strings.Index(text, verifyStart), strings.Index(text, verifyEnd)
	if start >= 0 && end > start {
		text = text[:start] + section + strings.TrimPrefix(text[end+len(verifyEnd):], "\n")
	} else {
		text = strings.TrimRight(text, "\n") + "\n\n" + section
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err == nil {
		fmt.Printf("Guide updated: %s\n", path)
	}
}

// verifyCommand implements `gclone verify [directory]`.
func VerifyCommand(abs string, o project.Options) error {
	p := analyze.AnalyzeProject(abs)
	if !p.HasUnits() {
		return fmt.Errorf("no Node.js, Go or Java project found in %s, nothing to verify", abs)
	}
	rep := RunVerify(p, o)
	updateGuideVerification(abs, rep)
	if rep.Verdict == "NOT READY" || rep.Verdict == "INTERRUPTED" {
		return errNotReady
	}
	return nil
}
