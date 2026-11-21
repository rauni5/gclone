package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rauni5/gclone/internal/doctor"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/state"
)

func TestParseCIAndJSON(t *testing.T) {
	o, err := parseArgs([]string{"user/app", "--ci", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !o.CI || !o.Yes || !o.JSON {
		t.Errorf("--ci must imply --yes: %+v", o)
	}
	o, _ = parseArgs([]string{"status", "--json"})
	if !o.StatusCmd || !o.JSON || o.CI {
		t.Errorf("%+v", o)
	}
}

func TestBuildResultOK(t *testing.T) {
	p := &project.Project{Commit: "abc"}
	p.Steps = []project.Step{{Name: "Install", Status: "ok"}}
	if r := BuildResult("/x", p, nil); !r.OK || r.Commit != "abc" {
		t.Errorf("%+v", r)
	}
	p.Steps = append(p.Steps, project.Step{Name: "Build", Status: "failed", Note: "boom"})
	if r := BuildResult("/x", p, nil); r.OK {
		t.Error("a failed step must make ok false")
	}
	p.Steps = p.Steps[:1]
	p.Verify = &project.Report{Verdict: "NOT READY"}
	if r := BuildResult("/x", p, nil); r.OK || r.Verify == nil || r.Verify.Verdict != "NOT READY" {
		t.Errorf("%+v", r)
	}
	if r := BuildResult("", nil, errors.New("git clone failed")); r.OK || r.Error == "" {
		t.Errorf("%+v", r)
	}
}

func TestResultIsValidJSONWithEmptyLists(t *testing.T) {
	var b bytes.Buffer
	if err := WriteResult(&b, BuildResult("/x", &project.Project{}, nil)); err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"detected", "warnings", "steps", "services"} {
		if v, ok := m[k].([]interface{}); !ok || v == nil {
			t.Errorf("%s must be an empty list, got %v", k, m[k])
		}
	}
}

func TestCIFailure(t *testing.T) {
	p := &project.Project{}
	if ciFailure(p) != nil {
		t.Error("nothing failed")
	}
	p.Steps = []project.Step{{Status: "failed"}}
	if ciFailure(p) == nil {
		t.Error("failed step ignored")
	}
	p.Steps = nil
	p.Verify = &project.Report{Verdict: "INTERRUPTED"}
	if ciFailure(p) == nil {
		t.Error("bad verdict ignored")
	}
	p.Verify = &project.Report{Verdict: "READY WITH WARNINGS"}
	if ciFailure(p) != nil {
		t.Error("warnings alone must not fail")
	}
}

func TestStatusJSON(t *testing.T) {
	dir := t.TempDir()
	if _, err := statusJSON(&bytes.Buffer{}, dir); err == nil {
		t.Error("no state should be an error")
	}
	p := &project.Project{Commit: "abc", Steps: []project.Step{{Name: "Install", Status: "ok"}}}
	if err := state.Save(dir, state.FromProject(p)); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	ok, err := statusJSON(&b, dir)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var m map[string]interface{}
	if json.Unmarshal(b.Bytes(), &m) != nil || m["ok"] != true {
		t.Errorf("%s", b.String())
	}
	p.Steps[0].Status = "failed"
	_ = state.Save(dir, state.FromProject(p))
	if ok, _ := statusJSON(&bytes.Buffer{}, dir); ok {
		t.Error("failed step must give ok false")
	}
	_ = os.Remove(filepath.Join(dir, state.Dir))
}

func fakeEnv(haveGit bool) doctor.Env {
	return doctor.Env{
		Have:        func(b string) bool { return b != "git" || haveGit },
		Version:     func(string, ...string) string { return "1.0" },
		DockerState: func() string { return "ready" },
		PortFree:    func(int) bool { return true },
		Reachable:   func(int) bool { return false },
		FreeBytes:   func(string) (uint64, error) { return 50 << 30, nil },
		VenvWorks:   func() bool { return true },
	}
}

func TestDoctorEmptyFolder(t *testing.T) {
	var b bytes.Buffer
	ok, err := runDoctor(&b, t.TempDir(), false, fakeEnv(true))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v\n%s", ok, err, b.String())
	}
	if !strings.Contains(b.String(), "no supported project") || !strings.Contains(b.String(), "READY WITH WARNINGS") {
		t.Errorf("%s", b.String())
	}
}

func TestDoctorJSONAndFailure(t *testing.T) {
	var b bytes.Buffer
	ok, err := runDoctor(&b, t.TempDir(), true, fakeEnv(false))
	if err != nil || ok {
		t.Fatalf("a missing git must give ok=false: ok=%v err=%v", ok, err)
	}
	var r DoctorResult
	if err := json.Unmarshal(b.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.OK || r.Verdict != "NOT READY" || len(r.Checks) == 0 {
		t.Errorf("%+v", r)
	}
}

func TestDoctorNotADirectory(t *testing.T) {
	if _, err := runDoctor(&bytes.Buffer{}, filepath.Join(t.TempDir(), "nope"), false, fakeEnv(true)); err == nil {
		t.Error("a missing folder must be an error")
	}
}

func TestParseDoctor(t *testing.T) {
	o, _ := parseArgs([]string{"doctor", "app", "--json"})
	if !o.DoctorCmd || o.Dir != "app" || !o.JSON {
		t.Errorf("%+v", o)
	}
}

// TestVerifyJSONRoundTrip verifies that a verification report round-trips through
// the --json output for READY, READY WITH WARNINGS and NOT READY. It checks the
// schema version, the verify block and the ok flag, so the output stays machine
// readable for the three verdicts the issue names.
func TestVerifyJSONRoundTrip(t *testing.T) {
	cases := []struct {
		verdict string
		ok      bool
	}{
		{"READY", true},
		{"READY WITH WARNINGS", true},
		{"NOT READY", false},
	}
	for _, c := range cases {
		t.Run(c.verdict, func(t *testing.T) {
			checks := []project.Check{
				{Name: "env", Status: "skip"},
				{Name: "start", Status: "pass", Detail: "listening"},
			}
			if c.verdict != "READY" {
				checks = append(checks, project.Check{Name: "http", Status: "warn", Detail: "slow"})
			}
			p := &project.Project{
				Dir: "/x",
				Verify: &project.Report{
					Dir:     "/x",
					Verdict: c.verdict,
					Units:   []project.UnitReport{{Label: ". (Python)", Checks: checks}},
				},
			}
			var b bytes.Buffer
			if err := WriteResult(&b, BuildResult("/x", p, nil)); err != nil {
				t.Fatal(err)
			}
			var got Result
			if err := json.Unmarshal(b.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, b.String())
			}
			if got.Schema != ResultSchema {
				t.Errorf("schema = %d, want %d", got.Schema, ResultSchema)
			}
			if got.Verify == nil {
				t.Fatal("verify block is missing")
			}
			if got.Verify.Verdict != c.verdict {
				t.Errorf("verdict = %q, want %q", got.Verify.Verdict, c.verdict)
			}
			if got.OK != c.ok {
				t.Errorf("ok = %v, want %v", got.OK, c.ok)
			}
			if len(got.Verify.Units) != 1 || len(got.Verify.Units[0].Checks) != len(checks) {
				t.Errorf("units = %+v", got.Verify.Units)
			}
		})
	}
}
