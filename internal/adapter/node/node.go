package node

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/services"
)

type packageJSON struct {
	Name           string                 `json:"name"`
	Scripts        map[string]string      `json:"scripts"`
	Deps           map[string]string      `json:"dependencies"`
	DevDeps        map[string]string      `json:"devDependencies"`
	Engines        map[string]interface{} `json:"engines"`
	PackageManager string                 `json:"packageManager"`
	Workspaces     json.RawMessage        `json:"workspaces"`
}

type NodeInfo struct {
	Version string
	Major   int
}

var IntRe = regexp.MustCompile(`\d+`)

func firstInt(s string) int {
	n, _ := strconv.Atoi(IntRe.FindString(s))
	return n
}

func InstalledNode() NodeInfo {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return NodeInfo{}
	}
	v := strings.TrimSpace(string(out))
	return NodeInfo{Version: v, Major: firstInt(v)}
}

func FindPackageDirs(root string) []string {
	var dirs []string
	if fsx.FileExists(filepath.Join(root, "package.json")) {
		dirs = append(dirs, root)
		if hasWorkspaces(root) {
			return dirs
		}
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || fsx.SkipDirs[e.Name()] {
			continue
		}
		d := filepath.Join(root, e.Name())
		if fsx.FileExists(filepath.Join(d, "package.json")) {
			dirs = append(dirs, d)
		}
		if len(dirs) >= 6 {
			break
		}
	}
	return dirs
}

func hasWorkspaces(dir string) bool {
	if fsx.FileExists(filepath.Join(dir, "pnpm-workspace.yaml")) {
		return true
	}
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pj packageJSON
	if json.Unmarshal(b, &pj) != nil {
		return false
	}
	return len(pj.Workspaces) > 0
}

var frameworks = []struct{ dep, name, port string }{
	{"next", "Next.js", "3000"},
	{"nuxt", "Nuxt", "3000"},
	{"@nestjs/core", "NestJS", "3000"},
	{"@sveltejs/kit", "SvelteKit", "5173"},
	{"astro", "Astro", "4321"},
	{"@remix-run/node", "Remix", "3000"},
	{"@angular/core", "Angular", "4200"},
	{"vite", "Vite", "5173"},
	{"react-scripts", "Create React App", "3000"},
	{"fastify", "Fastify", ""},
	{"express", "Express", ""},
	{"koa", "Koa", ""},
	{"vue", "Vue", ""},
	{"react", "React", ""},
}

var dbDeps = map[string]string{
	"pg": "PostgreSQL", "postgres": "PostgreSQL",
	"mysql": "MySQL", "mysql2": "MySQL",
	"mongodb": "MongoDB", "mongoose": "MongoDB",
	"redis": "Redis", "ioredis": "Redis",
	"sqlite3": "SQLite", "better-sqlite3": "SQLite",
	"prisma": "Prisma ORM", "@prisma/client": "Prisma ORM",
	"typeorm": "TypeORM", "sequelize": "Sequelize", "knex": "Knex", "drizzle-orm": "Drizzle ORM",
}

func AnalyzePkg(root, dir string, inst NodeInfo) (*project.Pkg, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}
	var pj packageJSON
	if err := json.Unmarshal(data, &pj); err != nil {
		return nil, fmt.Errorf("invalid package.json: %v", err)
	}
	pkg := &project.Pkg{Dir: dir, Rel: fsx.Rel(root, dir), Name: pj.Name, Scripts: pj.Scripts}
	if pkg.Name == "" {
		pkg.Name = filepath.Base(dir)
	}

	deps := map[string]bool{}
	for k := range pj.Deps {
		deps[k] = true
	}
	for k := range pj.DevDeps {
		deps[k] = true
	}

	pkg.PM, pkg.HasLock = detectPM(dir, pj.PackageManager)

	for _, f := range frameworks {
		if deps[f.dep] {
			pkg.Framework, pkg.Port = f.name, f.port
			break
		}
	}
	var hints []string
	seen := map[string]bool{}
	for dep, label := range dbDeps {
		if deps[dep] && !seen[label] {
			seen[label] = true
			hints = append(hints, label)
		}
	}
	sort.Strings(hints)
	pkg.DBHints = hints
	pkg.Prisma = deps["prisma"] || deps["@prisma/client"] || fsx.FileExists(filepath.Join(dir, "prisma", "schema.prisma"))

	pkg.LocalDeps = map[string]string{}
	for _, group := range []map[string]string{pj.Deps, pj.DevDeps} {
		for name, spec := range group {
			for _, pre := range []string{"file:", "link:"} {
				if path := strings.TrimPrefix(spec, pre); path != spec && strings.HasPrefix(path, "..") {
					pkg.LocalDeps[name] = path
				}
			}
		}
	}

	pkg.DepKinds = map[string]string{}
	names := make([]string, 0, len(deps))
	for d := range deps {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, d := range names {
		if k, ok := services.DepKind[d]; ok && pkg.DepKinds[k] == "" {
			pkg.DepKinds[k] = d
		}
	}
	if pkg.Prisma {
		if k := services.PrismaKind(dir); k != "" && pkg.DepKinds[k] == "" {
			pkg.DepKinds[k] = "prisma"
		}
	}

	for _, s := range []string{"preinstall", "install", "postinstall", "prepare"} {
		if c, ok := pj.Scripts[s]; ok {
			pkg.LifecycleScripts = append(pkg.LifecycleScripts, s+": "+c)
		}
	}

	for _, n := range envfile.EnvFileNames {
		if fsx.FileExists(filepath.Join(dir, n)) {
			pkg.EnvExample = n
			pkg.EnvVars = envfile.ParseEnvFile(filepath.Join(dir, n))
			break
		}
	}
	if pkg.PM != "" && pkg.EnvExample == "" && fsx.FileExists(filepath.Join(dir, ".env")) {
		pkg.EnvVars = envfile.ParseEnvFile(filepath.Join(dir, ".env"))
	}

	pkg.NodeWanted, pkg.NodeSource = wantedNode(root, dir, pj)
	checkNode(pkg, inst)
	return pkg, nil
}

func detectPM(dir, field string) (string, bool) {
	locks := []struct{ file, pm string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"bun.lock", "bun"},
		{"package-lock.json", "npm"},
		{"npm-shrinkwrap.json", "npm"},
	}
	pm := ""
	if i := strings.Index(field, "@"); i > 0 {
		pm = field[:i]
	}
	hasLock := false
	for _, l := range locks {
		if fsx.FileExists(filepath.Join(dir, l.file)) {
			if pm == "" {
				pm = l.pm
			}
			if l.pm == pm {
				hasLock = true
			}
		}
	}
	if pm == "" {
		pm = "npm"
	}
	return pm, hasLock
}

var toolVersionsNode = regexp.MustCompile(`^(?:nodejs|node)\s+(\S+)`)

func wantedNode(root, dir string, pj packageJSON) (string, string) {
	seen := map[string]bool{}
	for _, d := range []string{dir, root} {
		if seen[d] {
			continue
		}
		seen[d] = true
		for _, name := range []string{".nvmrc", ".node-version"} {
			if b, err := os.ReadFile(filepath.Join(d, name)); err == nil {
				if v := strings.TrimSpace(string(b)); v != "" {
					return v, name
				}
			}
		}
		if b, err := os.ReadFile(filepath.Join(d, ".tool-versions")); err == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if m := toolVersionsNode.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
					return m[1], ".tool-versions"
				}
			}
		}
	}
	if v, ok := pj.Engines["node"].(string); ok && v != "" {
		return v, "package.json engines"
	}
	return "", ""
}

func checkNode(pkg *project.Pkg, inst NodeInfo) {
	if inst.Major == 0 {
		pkg.Warnings = append(pkg.Warnings, "Node.js was not found in PATH. Install it (the guide suggests a version manager) before running anything.")
		return
	}
	want := firstInt(pkg.NodeWanted)
	if want == 0 {
		return
	}
	isMin := strings.Contains(pkg.NodeWanted, ">=") && !strings.Contains(pkg.NodeWanted, "<")
	if (isMin && inst.Major < want) || (!isMin && inst.Major != want) {
		pkg.Warnings = append(pkg.Warnings, fmt.Sprintf(
			"This project asks for Node %s (from %s) but you have %s. Switch with nvm, fnm or mise if you hit errors.",
			pkg.NodeWanted, pkg.NodeSource, inst.Version))
	}
}

func InstallCmd(pkg *project.Pkg, ignoreScripts bool) (string, []string) {
	var args []string
	switch pkg.PM {
	case "npm":
		if pkg.HasLock {
			args = []string{"ci"}
		} else {
			args = []string{"install"}
		}
	default:
		args = []string{"install"}
	}
	if ignoreScripts {
		args = append(args, "--ignore-scripts")
	}
	return pkg.PM, args
}

func Runnables(p *project.Project, noBuild bool) []project.Runnable {
	var out []project.Runnable
	for _, pkg := range p.Pkgs {
		r := project.Runnable{Label: pkg.Rel, Dir: pkg.Dir, ExampleName: pkg.EnvExample, Missing: pkg.MissingVars}
		if _, ok := pkg.Scripts["build"]; ok && !noBuild {
			r.Build = []string{pkg.PM, "run", "build"}
		}
		if s := project.DevScript(pkg); s != "" {
			r.Start = []string{pkg.PM, "run", s}
		}

		envPath := filepath.Join(pkg.Dir, ".env")
		if fsx.FileExists(envPath) {
			r.HasEnv = true
			for _, v := range envfile.ParseEnvFile(envPath) {
				if v.Kind == "blocking" {
					r.Blocking = append(r.Blocking, v.Key)
				}
				if v.Key == "PORT" && r.Port == 0 {
					if n, err := strconv.Atoi(v.Value); err == nil && n > 0 {
						r.Port, r.PortFrom = n, "PORT in .env"
					}
				}
			}
		}
		if r.Port == 0 {
			if n, err := strconv.Atoi(pkg.Port); err == nil && n > 0 {
				r.Port, r.PortFrom = n, pkg.Framework+" default"
			}
		}
		out = append(out, r)
	}
	return out
}
