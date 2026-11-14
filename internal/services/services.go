package services

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rauni5/gclone/internal/docker"
	"github.com/rauni5/gclone/internal/explain"
	"github.com/rauni5/gclone/internal/installer"
	"github.com/rauni5/gclone/internal/proc"
	"github.com/rauni5/gclone/internal/project"
	"github.com/rauni5/gclone/internal/ui"
)

type svcDef struct {
	Name  string
	Port  int
	Bins  []string
	Image string
	Data  string
}

var svcDefs = map[string]svcDef{
	"mongodb":  {"MongoDB", 27017, []string{"mongod"}, "mongo:7", "/data/db"},
	"postgres": {"PostgreSQL", 5432, []string{"postgres", "psql"}, "postgres:16", "/var/lib/postgresql/data"},
	"mysql":    {"MySQL", 3306, []string{"mysqld", "mysql"}, "mysql:8", "/var/lib/mysql"},
	"redis":    {"Redis", 6379, []string{"redis-server"}, "redis:7", "/data"},
	"rabbitmq": {"RabbitMQ", 5672, []string{"rabbitmq-server"}, "rabbitmq:3", "/var/lib/rabbitmq"},
}

var kindOrder = []string{"mongodb", "postgres", "mysql", "redis", "rabbitmq"}

var DepKind = map[string]string{
	"pg": "postgres", "postgres": "postgres", "pg-promise": "postgres",
	"mysql": "mysql", "mysql2": "mysql",
	"mongodb": "mongodb", "mongoose": "mongodb",
	"redis": "redis", "ioredis": "redis",
	"amqplib": "rabbitmq",
}

var prismaProvider = regexp.MustCompile(`datasource\s+\w+\s*\{[^}]*provider\s*=\s*"(\w+)"`)

func PrismaKind(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "prisma", "schema.prisma"))
	if err != nil {
		return ""
	}
	m := prismaProvider.FindStringSubmatch(string(b))
	if m == nil {
		return ""
	}
	switch m[1] {
	case "postgresql", "postgres":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "mongodb":
		return "mongodb"
	}
	return ""
}

var urlSchemeRe = regexp.MustCompile(`^(postgres(?:ql)?|mysql|mariadb|mongodb(?:\+srv)?|rediss?|amqps?)://`)

func schemeKind(scheme string) string {
	switch strings.ToLower(scheme) {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	case "mongodb", "mongodb+srv":
		return "mongodb"
	case "redis", "rediss":
		return "redis"
	case "amqp", "amqps":
		return "rabbitmq"
	}
	return ""
}

// parseServiceURL returns the service kind and the parsed URL. The URL is nil
// when the value looks like a service URL but cannot be parsed (for example a
// MongoDB replica set list).
func parseServiceURL(val string) (string, *url.URL) {
	m := urlSchemeRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(val)))
	if m == nil {
		return "", nil
	}
	kind := schemeKind(m[1])
	u, err := url.Parse(strings.TrimSpace(val))
	if err != nil {
		return kind, nil
	}
	return kind, u
}

func IsLocalHost(h string) bool {
	switch strings.ToLower(h) {
	case "", "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	}
	return false
}

func DetectNeeds(p *project.Project) []*project.Need {
	byKind := map[string]*project.Need{}
	localSeen := map[string]bool{}
	get := func(kind string) *project.Need {
		n := byKind[kind]
		if n == nil {
			d := svcDefs[kind]
			n = &project.Need{Kind: kind, Name: d.Name, Port: d.Port}
			byKind[kind] = n
		}
		return n
	}
	addReason := func(n *project.Need, r string) {
		for _, x := range n.Reasons {
			if x == r {
				return
			}
		}
		n.Reasons = append(n.Reasons, r)
	}

	scan := func(rel string, depKinds map[string]string, vars []project.EnvVar) {
		for _, kind := range kindOrder {
			if dep, ok := depKinds[kind]; ok {
				addReason(get(kind), fmt.Sprintf("%s: %s dependency", rel, dep))
			}
		}
		for _, v := range vars {
			kind, u := parseServiceURL(strings.TrimPrefix(v.Value, "jdbc:"))
			if kind == "" {
				continue
			}
			n := get(kind)
			addReason(n, fmt.Sprintf("%s: %s", rel, v.Key))
			if u == nil || v.Kind == "blocking" {
				continue
			}
			host := u.Hostname()
			scheme := strings.ToLower(u.Scheme)
			if !IsLocalHost(host) || strings.HasSuffix(scheme, "+srv") || scheme == "rediss" {
				if !localSeen[kind] {
					n.Remote, n.RemoteHost = true, host
				}
				continue
			}
			localSeen[kind] = true
			n.Remote, n.RemoteHost = false, ""
			if port, err := strconv.Atoi(u.Port()); err == nil && port > 0 {
				n.Port = port
			}
			if u.User != nil {
				n.User = u.User.Username()
				n.Pass, _ = u.User.Password()
			}
			n.DB = strings.TrimPrefix(u.Path, "/")
		}
	}
	for _, pkg := range p.Pkgs {
		scan(pkg.Rel, pkg.DepKinds, pkg.EnvVars)
	}
	for _, j := range p.JavaProjects {
		scan(j.Rel, j.DepKinds, j.Config)
		// Spring keeps the login apart from the JDBC URL.
		var user, pass string
		for _, v := range j.Config {
			switch v.Key {
			case "spring.datasource.username":
				user = v.Value
			case "spring.datasource.password":
				pass = v.Value
			}
		}
		for _, v := range j.Config {
			if v.Key != "spring.datasource.url" {
				continue
			}
			if kind, _ := parseServiceURL(strings.TrimPrefix(v.Value, "jdbc:")); kind != "" {
				if n := byKind[kind]; n != nil && !n.Remote && n.User == "" {
					n.User, n.Pass = user, pass
				}
			}
		}
	}

	var out []*project.Need
	for _, k := range kindOrder {
		if n := byKind[k]; n != nil {
			out = append(out, n)
		}
	}
	return out
}

var imageKindRe = regexp.MustCompile(`(?i)(?:^|/)(postgres|postgresql|mysql|mariadb|mongo|mongodb|redis|valkey|rabbitmq)(?::|@|$)`)

func imageKind(img string) string {
	m := imageKindRe.FindStringSubmatch(img)
	if m == nil {
		return ""
	}
	switch strings.ToLower(m[1]) {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "mongo", "mongodb":
		return "mongodb"
	case "redis", "valkey":
		return "redis"
	case "rabbitmq":
		return "rabbitmq"
	}
	return ""
}

// composeKinds lists the service kinds that docker compose files provide. With
// startedOnly it only counts files whose services were actually started.
func composeKinds(p *project.Project, startedOnly bool) map[string]bool {
	covered := map[string]bool{}
	for _, c := range p.Composes {
		if startedOnly && !c.Started {
			continue
		}
		for _, s := range c.DBServices {
			if k := imageKind(c.Images[s]); k != "" {
				covered[k] = true
			}
		}
	}
	return covered
}

func Reachable(port int) bool {
	// Dev servers that bind "localhost" may listen on IPv6 only, so try both.
	for _, host := range []string{"127.0.0.1", "::1"} {
		c, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 700*time.Millisecond)
		if err == nil {
			c.Close()
			return true
		}
	}
	return false
}

func waitReachable(port int, d time.Duration) bool {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		if Reachable(port) {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

func needStatus(p *project.Project, o project.Options, n *project.Need, covered map[string]bool) string {
	switch {
	case n.Remote:
		return "uses remote host " + n.RemoteHost + ", nothing to install"
	case covered[n.Kind] && !o.NoServices:
		s := "will come from docker compose"
		if note := docker.Note(p.DockerState); note != "" {
			s += " (" + note + ")"
		}
		return s
	case Reachable(n.Port):
		return "running"
	case proc.AnyBin(svcDefs[n.Kind].Bins):
		return "installed, but not running"
	}
	return "not found"
}

func PrintNeeds(p *project.Project, o project.Options) {
	if len(p.Needs) == 0 {
		return
	}
	covered := composeKinds(p, false)
	fmt.Println("\nServices the project needs:")
	for _, n := range p.Needs {
		fmt.Printf("  - %s (localhost:%d): %s\n", n.Name, n.Port, needStatus(p, o, n, covered))
		fmt.Printf("      why: %s\n", strings.Join(n.Reasons, ", "))
	}
	if note := docker.Note(p.DockerState); note != "" {
		fmt.Println("  Docker: " + note + ".")
	}
	fmt.Println("  You will be asked before anything is installed or started.")
}

func nativeOption(kind string) *installer.Option {
	switch runtime.GOOS {
	case "windows":
		ids := map[string]string{"mongodb": "MongoDB.Server", "postgres": "PostgreSQL.PostgreSQL.16", "mysql": "Oracle.MySQL"}
		if id, ok := ids[kind]; ok && proc.HasBin("winget") {
			return &installer.Option{
				Manager: "winget",
				Cmds:    [][]string{{"winget", "install", "-e", "--id", id, "--accept-package-agreements", "--accept-source-agreements"}},
				Note:    "Windows may ask for administrator permission.",
			}
		}
	case "darwin":
		formulas := map[string]string{"mongodb": "mongodb-community", "postgres": "postgresql@16", "mysql": "mysql", "redis": "redis"}
		f, ok := formulas[kind]
		if !ok || !proc.HasBin("brew") {
			return nil
		}
		var cmds [][]string
		if kind == "mongodb" {
			cmds = append(cmds, []string{"brew", "tap", "mongodb/brew"})
		}
		cmds = append(cmds, []string{"brew", "install", f}, []string{"brew", "services", "start", f})
		return &installer.Option{Manager: "Homebrew", Cmds: cmds}
	case "linux":
		pkgs := map[string]string{"postgres": "postgresql", "mysql": "mysql-server", "redis": "redis-server"}
		units := map[string]string{"postgres": "postgresql", "mysql": "mysql", "redis": "redis-server"}
		pkg, ok := pkgs[kind]
		if !ok || !proc.HasBin("apt-get") {
			return nil
		}
		prefix, ok := docker.SudoPrefix()
		if !ok {
			return nil
		}
		with := func(args ...string) []string { return append(append([]string{}, prefix...), args...) }
		return &installer.Option{
			Manager: "apt",
			Cmds: [][]string{
				with("apt-get", "update"),
				with("apt-get", "install", "-y", pkg),
				with("systemctl", "start", units[kind]),
			},
			Note: "This needs administrator rights.",
		}
	}
	return nil
}

func dockerRunArgs(n *project.Need, name string) []string {
	d := svcDefs[n.Kind]
	args := []string{"run", "-d", "--name", name, "-p", fmt.Sprintf("%d:%d", n.Port, d.Port), "-v", name + ":" + d.Data}
	switch n.Kind {
	case "postgres":
		pass := n.Pass
		if pass == "" {
			pass = "postgres"
		}
		args = append(args, "-e", "POSTGRES_PASSWORD="+pass)
		if n.User != "" {
			args = append(args, "-e", "POSTGRES_USER="+n.User)
		}
		if n.DB != "" {
			args = append(args, "-e", "POSTGRES_DB="+n.DB)
		}
	case "mysql":
		pass := n.Pass
		if n.User == "" || n.User == "root" {
			if pass == "" {
				pass = "root"
			}
			args = append(args, "-e", "MYSQL_ROOT_PASSWORD="+pass)
		} else {
			if pass == "" {
				pass = "password"
			}
			args = append(args, "-e", "MYSQL_RANDOM_ROOT_PASSWORD=yes", "-e", "MYSQL_USER="+n.User, "-e", "MYSQL_PASSWORD="+pass)
		}
		if n.DB != "" {
			args = append(args, "-e", "MYSQL_DATABASE="+n.DB)
		}
	}
	return append(args, d.Image)
}

func credsNote(n *project.Need) string {
	switch n.Kind {
	case "postgres":
		user, pass := n.User, n.Pass
		if user == "" {
			user = "postgres"
		}
		if pass == "" {
			pass = "postgres"
		}
		s := fmt.Sprintf("user %s, password %s", user, pass)
		if n.DB != "" {
			s += ", database " + n.DB
		}
		return s
	case "mysql":
		if n.User == "" || n.User == "root" {
			pass := n.Pass
			if pass == "" {
				pass = "root"
			}
			return "user root, password " + pass
		}
		pass := n.Pass
		if pass == "" {
			pass = "password"
		}
		return fmt.Sprintf("user %s, password %s", n.User, pass)
	}
	return ""
}

func runInDocker(n *project.Need) error {
	name := "gclone-" + n.Kind
	n.Container = name
	if exec.Command("docker", "container", "inspect", name).Run() == nil {
		return proc.RunStream("", false, "docker", "start", name)
	}
	if err := docker.EnsureImage(svcDefs[n.Kind].Image); err != nil {
		return err
	}
	return proc.RunStream("", false, "docker", dockerRunArgs(n, name)...)
}

func ProvisionServices(p *project.Project, o project.Options) {
	if len(p.Needs) == 0 {
		return
	}
	covered := composeKinds(p, true)
	fmt.Println("\nChecking the services this project needs...")
	for _, n := range p.Needs {
		switch {
		case n.Remote:
			n.Result = "configured for remote host " + n.RemoteHost + ", nothing to install"
		case covered[n.Kind]:
			n.Result = "provided by docker compose"
		case Reachable(n.Port):
			n.Result = fmt.Sprintf("already running on localhost:%d", n.Port)
		default:
			n.Result = offerInstall(p, o, n)
		}
		fmt.Printf("  %s: %s\n", n.Name, n.Result)
	}
}

type svcOption struct {
	label string
	cmd   string
	kind  string // "docker" or "native"
	heavy bool   // installs a large program (Docker itself), never approved automatically
	run   func() error
}

func offerInstall(p *project.Project, o project.Options, n *project.Need) string {
	d := svcDefs[n.Kind]
	installed := proc.AnyBin(d.Bins)
	var nat *installer.Option
	if !installed {
		nat = nativeOption(n.Kind)
	}
	state := docker.State()
	p.DockerState = state

	viaDocker := func(prep func() error) func() error {
		return func() error {
			if prep != nil {
				if err := prep(); err != nil {
					return err
				}
			}
			return runInDocker(n)
		}
	}
	dockerWhat := fmt.Sprintf("container gclone-%s, data kept in a volume", n.Kind)

	var opts []svcOption
	switch state {
	case "ready":
		opts = append(opts, svcOption{
			label: "Run it in Docker (" + dockerWhat + ")",
			cmd:   "docker run gclone-" + n.Kind, kind: "docker", run: viaDocker(nil),
		})
	case "stopped":
		opts = append(opts, svcOption{
			label: "Start Docker, then run it in Docker (" + dockerWhat + ")",
			cmd:   "start docker, docker run gclone-" + n.Kind, kind: "docker",
			run: viaDocker(func() error {
				if !docker.StartAndWait() {
					return fmt.Errorf("Docker did not start")
				}
				return nil
			}),
		})
	}
	if nat != nil {
		label := "Install it with " + nat.Manager + "."
		if nat.Note != "" {
			label += " " + nat.Note
		}
		opts = append(opts, svcOption{label: label, cmd: nat.Manager + " install", kind: "native", run: nat.Run})
	}
	if state == "missing" {
		if inst := docker.InstallOption(); inst != nil {
			opts = append(opts, svcOption{
				label: "Install Docker with " + inst.Manager + ", then run it in Docker. " + inst.Note,
				cmd:   inst.Manager + " install docker", kind: "docker", heavy: true,
				run: viaDocker(func() error {
					if err := inst.Run(); err != nil {
						return err
					}
					if !docker.StartAndWait() {
						return fmt.Errorf("Docker was installed but did not start")
					}
					return nil
				}),
			})
		}
	}

	fmt.Printf("\n%s is needed (%s) but nothing is listening on localhost:%d.\n", n.Name, strings.Join(n.Reasons, ", "), n.Port)
	if installed {
		fmt.Printf("  A local %s install was found. Start it yourself, or pick another option.\n", n.Name)
	}
	if state == "denied" {
		fmt.Println("  " + docker.DockerDeniedMsg)
	}
	if len(opts) == 0 {
		why := "no supported package manager was found"
		switch state {
		case "missing":
			why = "Docker is not installed and no supported installer was found"
		case "denied":
			why = "Docker is not usable for this user"
		}
		fmt.Println("  No automatic option is available here (" + why + ").")
		return "not running, no automatic option available, see the guide"
	}
	for i, op := range opts {
		fmt.Printf("  %d) %s\n", i+1, op.label)
	}
	fmt.Printf("  %d) Skip, I will set it up myself\n", len(opts)+1)

	pick := 0
	if o.InstallServices {
		for i, op := range opts {
			if !op.heavy {
				pick = i + 1
				fmt.Printf("  --install-services given, choosing option %d.\n", pick)
				break
			}
		}
		if pick == 0 {
			fmt.Println("  --install-services does not install Docker itself, skipping.")
		}
	} else {
		pick = ui.Choose(len(opts) + 1)
		if pick > len(opts) {
			pick = 0
		}
	}
	if pick == 0 {
		return "not running, skipped"
	}

	chosen := opts[pick-1]
	fmt.Printf("\n[..] %s\n", chosen.label)
	err := chosen.run()
	if err != nil && chosen.kind == "docker" && nat != nil {
		fmt.Printf("[!!] %v\n", err)
		if ui.AskYesNo(fmt.Sprintf("Docker did not work. Install %s with %s instead?", n.Name, nat.Manager), false) {
			chosen = svcOption{label: "native", cmd: nat.Manager + " install", kind: "native", run: nat.Run}
			err = chosen.run()
		}
	}
	step := project.Step{Where: "system", Name: n.Name + " (" + chosen.kind + ")", Cmd: chosen.cmd, Status: "ok"}
	if err != nil {
		step.Status, step.Note = "failed", err.Error()
		p.Steps = append(p.Steps, step)
		fmt.Printf("[!!] failed: %v\n", err)
		if h, ok := explain.FromError(err); ok {
			explain.Print(h)
			step.Hint = h.Line()
			p.Steps[len(p.Steps)-1].Hint = step.Hint
		}
		return "failed: " + err.Error()
	}
	p.Steps = append(p.Steps, step)

	up := waitReachable(n.Port, 30*time.Second)
	switch {
	case chosen.kind == "docker" && up:
		s := fmt.Sprintf("started in Docker (container %s) on localhost:%d", n.Container, n.Port)
		if c := credsNote(n); c != "" {
			s += "; " + c
		}
		return s
	case chosen.kind == "docker":
		return fmt.Sprintf("container %s started but localhost:%d is not answering yet, check `docker logs %s`", n.Container, n.Port, n.Container)
	case up:
		return fmt.Sprintf("installed with %s and running on localhost:%d", nat.Manager, n.Port)
	}
	return fmt.Sprintf("installed with %s, but nothing answers on localhost:%d yet. Start the service, then run gclone setup again", nat.Manager, n.Port)
}

// guideServices writes the services section of SETUP_GUIDE.md.
func GuideServices(b *strings.Builder, p *project.Project) {
	if len(p.Needs) == 0 {
		return
	}
	w := func(format string, a ...interface{}) { fmt.Fprintf(b, format+"\n", a...) }
	w("## Services this project needs")
	w("")
	for _, n := range p.Needs {
		res := n.Result
		if res == "" {
			res = "not checked"
		}
		w("- **%s** (localhost:%d): %s", n.Name, n.Port, res)
		w("  - why: %s", strings.Join(n.Reasons, ", "))
		if n.Container != "" {
			w("  - manage: `docker stop %s`, `docker start %s`; remove with `docker rm -f %s && docker volume rm %s`", n.Container, n.Container, n.Container, n.Container)
		}
	}
	w("")
}
