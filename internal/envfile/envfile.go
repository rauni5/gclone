package envfile

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
)

var EnvFileNames = []string{
	".env.example", ".env.sample", ".env.template", ".env.dist",
	".env.local.example", ".env.development.example", "example.env", "sample.env",
}

var envLine = regexp.MustCompile(`^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$`)

var placeholder = regexp.MustCompile(`(?i)^$|your[_\- ]|changeme|change[_-]me|replace|xxx|<[^>]*>|todo|^secret$|^password$|^token$|^key$|insert|put[_-]|\.\.\.|example\.(com|org)`)

var dbKey = regexp.MustCompile(`(?i)(database_url|^db_|_db_|_db$|mongo|postgres|^pg|mysql|redis|sqlite|supabase)`)

var secretKey = regexp.MustCompile(`(?i)(secret|token|passw|api[_-]?key|private[_-]?key|credential|auth[_-]?key|salt)`)

// classify decides how much attention a variable needs.
//
//	blocking: empty or placeholder, the app probably will not run until it is set
//	review:   has a working default, but should change for real use
//	ok:       default is fine
func classify(key, val string) (string, string) {
	v := strings.TrimSpace(val)
	if placeholder.MatchString(v) {
		if v == "" {
			return "blocking", "empty in the example file, needs a value"
		}
		return "blocking", "placeholder value, replace it with a real one"
	}
	if dbKey.MatchString(key) {
		return "review", "database setting, the default targets a local dev database; change it to use your own"
	}
	if secretKey.MatchString(key) {
		return "review", "secret, fine for local use, replace before sharing or deploying"
	}
	return "ok", ""
}

func ParseEnvFile(path string) []project.EnvVar {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var vars []project.EnvVar
	var comment []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			comment = nil
			continue
		}
		if strings.HasPrefix(line, "#") {
			comment = append(comment, strings.TrimSpace(strings.TrimLeft(line, "#")))
			continue
		}
		m := envLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		val := strings.TrimSpace(m[2])
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		} else if i := strings.Index(val, " #"); i >= 0 {
			val = strings.TrimSpace(val[:i])
		}
		kind, reason := classify(m[1], val)
		c := strings.Join(comment, " ")
		if kind == "blocking" && val == "" && strings.Contains(strings.ToLower(c), "optional") {
			kind, reason = "ok", "marked optional"
		}
		vars = append(vars, project.EnvVar{Key: m[1], Value: val, Comment: c, Kind: kind, Reason: reason})
		comment = nil
	}
	return vars
}

var envUseRe = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)`),
	regexp.MustCompile(`process\.env\[\s*['"]([A-Za-z0-9_]+)['"]\s*\]`),
	regexp.MustCompile(`import\.meta\.env\.([A-Z][A-Z0-9_]*)`),
}

var sourceExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true, ".mjs": true, ".cjs": true,
	".vue": true, ".svelte": true, ".astro": true,
}

var ignoredVars = map[string]bool{
	"NODE_ENV": true, "CI": true, "HOME": true, "PATH": true, "PWD": true,
	"USER": true, "TZ": true, "DEBUG": true,
}

// scanEnvUsage finds environment variables that the source code reads.
func ScanEnvUsage(dir string, exclude []string) []string {
	found := map[string]bool{}
	files := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (fsx.SkipDirs[name] || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			for _, ex := range exclude {
				if path == ex {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !sourceExt[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		if files++; files > 5000 {
			return filepath.SkipAll
		}
		if info, err := d.Info(); err != nil || info.Size() > 1<<20 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, re := range envUseRe {
			for _, m := range re.FindAllSubmatch(b, -1) {
				found[string(m[1])] = true
			}
		}
		return nil
	})
	var out []string
	for k := range found {
		if !ignoredVars[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func MissingVars(used []string, vars []project.EnvVar) []string {
	have := map[string]bool{}
	for _, v := range vars {
		have[v.Key] = true
	}
	var out []string
	for _, k := range used {
		if !have[k] {
			out = append(out, k)
		}
	}
	return out
}
