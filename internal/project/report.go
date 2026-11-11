package project

import (
	"fmt"
	"strings"
	"time"
)

// Check is the result of one verification check.
type Check struct {
	Name   string
	Status string // pass, warn, fail or skip
	Detail string
}

// UnitReport holds the checks for one runnable part of the repository.
type UnitReport struct {
	Label  string
	Checks []Check
}

// Report is the outcome of verifying a repository.
type Report struct {
	Dir      string
	When     time.Time
	Services []Check
	Units    []UnitReport
	Verdict  string
}

func (r *Report) Counts() (fail, warn int) {
	count := func(cs []Check) {
		for _, c := range cs {
			switch c.Status {
			case "fail":
				fail++
			case "warn":
				warn++
			}
		}
	}
	count(r.Services)
	for _, u := range r.Units {
		count(u.Checks)
	}
	return
}

func (r *Report) Summary() string {
	f, w := r.Counts()
	var parts []string
	if f > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", f))
	}
	if w > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", w))
	}
	if len(parts) == 0 {
		return r.Verdict
	}
	return r.Verdict + " (" + strings.Join(parts, ", ") + ")"
}
