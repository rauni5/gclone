package repos

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/adapter/node"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
	"github.com/rauni5/gclone/internal/ui"
)

var ghLink = regexp.MustCompile(`https?://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)

var companionName = regexp.MustCompile(`(?i)(back-?end|api|server|service)`)

var backendWord = regexp.MustCompile(`(?i)back-?end`)

var originRe = regexp.MustCompile(`github\.com[:/]([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

var apiVarRe = regexp.MustCompile(`(?i)((api|backend|server)[_-]?(url|uri|base|host|endpoint|origin))|^(vite|react_app|next_public|vue_app|nuxt_public)_.*(api|backend)`)

// frameworks that ship their own server, so an API URL is not a sign of a missing backend
var serverFrameworks = map[string]bool{
	"Express": true, "Fastify": true, "Koa": true, "NestJS": true,
	"Next.js": true, "Nuxt": true, "Remix": true, "SvelteKit": true, "Astro": true,
}

func ownerRepo(u string) string {
	m := originRe.FindStringSubmatch(strings.TrimSpace(u))
	if m == nil {
		return ""
	}
	return strings.ToLower(m[1] + "/" + m[2])
}

func readmeCompanions(root, self string) []*project.Companion {
	selfOwner := strings.SplitN(self, "/", 2)[0]
	entries, _ := os.ReadDir(root)
	var data []byte
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "readme") {
			if b, err := os.ReadFile(filepath.Join(root, e.Name())); err == nil {
				data = b
				break
			}
		}
	}
	seen := map[string]bool{}
	var out []*project.Companion
	for _, line := range strings.Split(string(data), "\n") {
		for _, m := range ghLink.FindAllStringSubmatch(line, -1) {
			repo := strings.TrimSuffix(strings.TrimRight(m[2], ".,"), ".git")
			key := strings.ToLower(m[1] + "/" + repo)
			if key == self || seen[key] {
				continue
			}
			sameOwner := selfOwner != "" && strings.EqualFold(m[1], selfOwner)
			mentionsBackend := backendWord.MatchString(line)
			if !((companionName.MatchString(repo) && (sameOwner || mentionsBackend)) || (mentionsBackend && sameOwner)) {
				continue
			}
			seen[key] = true
			out = append(out, &project.Companion{
				Name:   repo,
				URL:    "https://github.com/" + m[1] + "/" + repo + ".git",
				Reason: "linked in the README as a backend, API or server repository",
			})
			if len(out) >= 3 {
				return out
			}
		}
	}
	return out
}

func apiRefs(p *project.Project) []project.ApiRef {
	for _, pkg := range p.Pkgs {
		if serverFrameworks[pkg.Framework] {
			return nil
		}
	}
	var refs []project.ApiRef
	for _, pkg := range p.Pkgs {
		for _, v := range pkg.EnvVars {
			if !apiVarRe.MatchString(v.Key) {
				continue
			}
			u, err := url.Parse(v.Value)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !services.IsLocalHost(u.Hostname()) {
				continue
			}
			port, _ := strconv.Atoi(u.Port())
			refs = append(refs, project.ApiRef{Pkg: pkg.Rel, Key: v.Key, Value: v.Value, Port: port})
		}
	}
	return refs
}

func DetectCompanions(p *project.Project) []*project.Companion {
	self := ownerRepo(proc.GitOutput(p.Dir, "remote", "get-url", "origin"))
	var comps []*project.Companion
	seen := map[string]bool{}
	add := func(c *project.Companion) {
		key := strings.ToLower(c.URL)
		if key == "" {
			key = "name:" + strings.ToLower(c.Name)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		comps = append(comps, c)
	}

	for _, c := range readmeCompanions(p.Dir, self) {
		add(c)
	}
	for _, pkg := range p.Pkgs {
		names := make([]string, 0, len(pkg.LocalDeps))
		for n := range pkg.LocalDeps {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			rel := pkg.LocalDeps[name]
			target := filepath.Clean(filepath.Join(pkg.Dir, rel))
			if !fsx.IsInside(p.Dir, target) && target != p.Dir && !fsx.FileExists(target) {
				add(&project.Companion{Name: filepath.Base(target), Reason: fmt.Sprintf("%s: dependency %q points to %s, which is not on this machine", pkg.Rel, name, rel)})
			}
		}
	}
	for _, c := range p.Composes {
		for _, out := range c.Outside {
			target := filepath.Clean(filepath.Join(filepath.Dir(c.Path), out))
			if !fsx.IsInside(p.Dir, target) && target != p.Dir && !fsx.FileExists(target) {
				add(&project.Companion{Name: filepath.Base(target), Reason: fmt.Sprintf("%s builds from %s, which is not on this machine", c.Rel, out)})
			}
		}
	}
	if refs := apiRefs(p); len(refs) > 0 {
		if len(comps) > 0 {
			comps[0].Refs = refs
			comps[0].Reason += fmt.Sprintf("; the app calls a local backend through %s=%s", refs[0].Key, refs[0].Value)
		} else {
			add(&project.Companion{
				Name:   "backend",
				Reason: fmt.Sprintf("%s=%s points at a local backend, but this repository has no server of its own", refs[0].Key, refs[0].Value),
				Refs:   refs,
			})
		}
	}
	return comps
}

// initSubmodules fetches git submodules after asking. It returns warnings for the guide.
func InitSubmodules(dir string, o project.Options) []string {
	const manual = "Run `git submodule update --init --recursive` to fetch the git submodules."
	if o.DryRun {
		fmt.Println("\nThis repository uses git submodules, which would be fetched (after asking).")
		return nil
	}
	if !o.Yes && !ui.AskYesNo("\nThis repository uses git submodules (other repositories inside it). Fetch them?", true) {
		return []string{"Git submodules were not fetched. " + manual}
	}
	fmt.Println("Fetching git submodules...")
	if err := proc.RunStream(dir, true, "git", "submodule", "update", "--init", "--recursive"); err != nil {
		return []string{"Some git submodules could not be fetched (private repository or no access?). " + manual}
	}
	return nil
}

func PrintCompanions(p *project.Project) {
	if len(p.Companions) == 0 {
		return
	}
	fmt.Println("\nOther repositories this project seems to depend on:")
	for _, c := range p.Companions {
		line := "  - " + c.Name + ": " + c.Reason
		if c.URL != "" {
			line += " (" + strings.TrimSuffix(c.URL, ".git") + ")"
		}
		fmt.Println(line)
	}
	fmt.Println("  You will be asked before anything is cloned.")
}

// handleCompanions asks about each companion repository, then clones and sets it up.
// SetupFunc sets up one repository that is already on disk. The CLI supplies it, so this
// package does not depend on the CLI.
type SetupFunc func(dir string, o project.Options, depth int) (*project.Project, error)

func HandleCompanions(p *project.Project, o project.Options, setup SetupFunc) {
	if len(p.Companions) == 0 {
		return
	}
	fmt.Println("\nThis project also seems to need other repositories.")
	parent := filepath.Dir(p.Dir)
	for _, c := range p.Companions {
		fmt.Printf("\n- %s: %s\n", c.Name, c.Reason)
		if c.URL == "" {
			fmt.Print("  Paste its repository URL (or user/repo) to set it up too, or press Enter to skip: ")
			line, _ := ui.StdinReader.ReadString('\n')
			if line = strings.TrimSpace(line); line != "" {
				if u := ExpandURL(line); looksLikeRepo(u) {
					c.URL = u
					c.Name = RepoName(u)
				} else {
					fmt.Println("  That does not look like a repository URL, user/repo or local path, skipping.")
				}
			}
		}
		if c.URL == "" {
			c.Result = "not set up, no repository URL known"
			continue
		}
		target := filepath.Join(parent, RepoName(c.URL))
		c.Dir = target
		exists := fsx.FileExists(target)
		q := fmt.Sprintf("  Clone %s into %s and set it up too?", strings.TrimSuffix(c.URL, ".git"), target)
		if exists {
			q = fmt.Sprintf("  %s already exists. Set it up too?", target)
		}
		if !o.CloneCompanions && !ui.AskYesNo(q, false) {
			c.Result = "skipped"
			continue
		}
		if !exists {
			fmt.Printf("Cloning %s\n", c.URL)
			if err := proc.RunStream("", true, "git", "clone", c.URL, target); err != nil {
				c.Result = "clone failed (private repository or no access?): " + err.Error()
				fmt.Println("[!!] " + c.Result)
				continue
			}
		}
		co := o
		co.Yes = o.Yes && o.CloneCompanions // with plain --yes, still confirm the plan of an extra repository
		cp, err := setup(target, co, 1)
		c.Project = cp
		switch {
		case err != nil:
			c.Result = "set up failed: " + err.Error()
		case cp != nil && cp.Executed && exists:
			c.Result = "set up in the existing folder " + target
		case cp != nil && cp.Executed:
			c.Result = "cloned to " + target + " and set up"
		case exists:
			c.Result = "found at " + target + ", setup not run"
		default:
			c.Result = "cloned to " + target + ", setup not run"
		}
	}
}

func looksLikeRepo(u string) bool {
	return strings.Contains(u, "://") || strings.HasPrefix(u, "git@") || fsx.FileExists(u)
}

func portNum(s string) int {
	n, _ := strconv.Atoi(node.IntRe.FindString(s))
	return n
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func GuideCompanions(b *strings.Builder, p *project.Project) {
	if len(p.Companions) == 0 {
		return
	}
	w := func(format string, a ...interface{}) { fmt.Fprintf(b, format+"\n", a...) }
	w("## Other repositories this project depends on")
	w("")
	for _, c := range p.Companions {
		title := c.Name
		if c.URL != "" {
			title += " (" + strings.TrimSuffix(c.URL, ".git") + ")"
		}
		res := c.Result
		if res == "" {
			res = "not checked"
		}
		w("- **%s**: %s", title, res)
		w("  - why: %s", c.Reason)
		for _, r := range c.Refs {
			w("  - this project calls it through `%s=%s`", r.Key, r.Value)
		}
		if c.Project == nil {
			continue
		}
		if len(c.Project.Pkgs) == 0 {
			extra := ""
			if len(c.Project.OtherStacks) > 0 {
				extra = " It looks like: " + strings.Join(c.Project.OtherStacks, "; ") + "."
			}
			w("  - gclone has no setup for its stack yet, follow its README.%s", extra)
			continue
		}
		var theirs []int
		for _, pkg := range c.Project.Pkgs {
			if s := project.DevScript(pkg); s != "" {
				path := filepath.ToSlash(filepath.Join("..", filepath.Base(c.Dir), pkg.Rel))
				w("  - start it: `cd %s && %s run %s`", path, pkg.PM, s)
			}
			if n := portNum(project.PortFor(pkg)); n > 0 {
				theirs = append(theirs, n)
			}
		}
		for _, r := range c.Refs {
			if r.Port > 0 && len(theirs) > 0 && !containsInt(theirs, r.Port) {
				w("  - **check the port**: `%s` uses port %d, but that repository listens on %s. Change one of them.", r.Key, r.Port, joinInts(theirs))
			}
		}
		for _, pkg := range p.Pkgs {
			if n := portNum(project.PortFor(pkg)); n > 0 && containsInt(theirs, n) {
				w("  - **port clash**: both `%s` and that repository want port %d. Change `PORT` in one of them.", pkg.Rel, n)
			}
		}
	}
	w("")
	w("Start the other repositories first, then this one.")
	w("")
}

func joinInts(xs []int) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = strconv.Itoa(x)
	}
	return strings.Join(s, ", ")
}

var shorthand = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func ExpandURL(s string) string {
	if fsx.FileExists(s) {
		return s
	}
	if shorthand.MatchString(s) {
		return "https://github.com/" + s + ".git"
	}
	return s
}

func RepoName(u string) string {
	u = strings.TrimRight(u, "/\\")
	u = strings.TrimSuffix(u, ".git")
	if i := strings.LastIndexAny(u, "/\\:"); i >= 0 {
		u = u[i+1:]
	}
	if u == "" {
		u = "repo"
	}
	return u
}
