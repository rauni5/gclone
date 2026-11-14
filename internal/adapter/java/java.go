// Package java is the Java adapter for Maven and Gradle projects, with extra knowledge of
// Spring Boot: required Java version, build tool and wrapper, application config and the
// database a Spring Boot app expects.
package java

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/rauni5/gclone/internal/envfile"
	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
)

var buildFiles = []string{"pom.xml", "build.gradle", "build.gradle.kts"}

func hasBuildFile(dir string) bool {
	for _, f := range buildFiles {
		if fsx.FileExists(filepath.Join(dir, f)) {
			return true
		}
	}
	return false
}

// FindProjects returns the directories that hold a Maven or Gradle build: just the root when
// it has one, otherwise folders one level down (for example backend/).
func FindProjects(root string) []string {
	if hasBuildFile(root) {
		return []string{root}
	}
	var dirs []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || fsx.SkipDirs[e.Name()] {
			continue
		}
		if d := filepath.Join(root, e.Name()); hasBuildFile(d) {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

var javaVersionRe = regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)

// ParseJavaMajor reads the output of `java -version` and returns the major version
// (8 for "1.8.0_392", 17 for "17.0.9"), or 0 when it cannot tell.
func ParseJavaMajor(out string) int {
	m := javaVersionRe.FindStringSubmatch(out)
	if m == nil {
		return 0
	}
	major, _ := strconv.Atoi(m[1])
	if major == 1 && m[2] != "" {
		major, _ = strconv.Atoi(m[2])
	}
	return major
}

// InstalledJava returns the installed major Java version, or 0 when java is missing.
func InstalledJava() int {
	out, err := exec.Command("java", "-version").CombinedOutput()
	if err != nil {
		return 0
	}
	return ParseJavaMajor(string(out))
}

// majorOf turns "17", "1.8", "1_8" or "11.0" into a major version.
func majorOf(s string) int {
	s = strings.NewReplacer("_", ".", "'", "", `"`, "").Replace(strings.TrimSpace(s))
	parts := strings.Split(s, ".")
	n, _ := strconv.Atoi(parts[0])
	if n == 1 && len(parts) > 1 {
		n, _ = strconv.Atoi(parts[1])
	}
	return n
}

var (
	mavenVersionRes = []*regexp.Regexp{
		regexp.MustCompile(`<java\.version>\s*([\d._]+)\s*</java\.version>`),
		regexp.MustCompile(`<maven\.compiler\.release>\s*([\d._]+)\s*</maven\.compiler\.release>`),
		regexp.MustCompile(`<maven\.compiler\.source>\s*([\d._]+)\s*</maven\.compiler\.source>`),
		regexp.MustCompile(`<release>\s*([\d._]+)\s*</release>`),
	}
	gradleVersionRes = []*regexp.Regexp{
		regexp.MustCompile(`JavaLanguageVersion\.of\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`jvmToolchain\(\s*(\d+)\s*\)`),
		regexp.MustCompile(`sourceCompatibility\s*=?\s*(?:JavaVersion\.VERSION_)?['"]?([\d._]+)`),
	}
	mavenBootRe  = regexp.MustCompile(`spring-boot-starter-parent|spring-boot-maven-plugin|spring-boot-dependencies`)
	gradleBootRe = regexp.MustCompile(`org\.springframework\.boot`)
	modulesRe    = regexp.MustCompile(`<modules>\s*<module>`)
	depKindRes   = []struct {
		kind string
		re   *regexp.Regexp
	}{
		{"postgres", regexp.MustCompile(`(?i)org\.postgresql|postgresql`)},
		{"mysql", regexp.MustCompile(`(?i)mysql-connector|mysql:mysql|mariadb-java-client`)},
		{"mongodb", regexp.MustCompile(`(?i)data-mongodb|mongodb-driver|mongo-java-driver`)},
		{"redis", regexp.MustCompile(`(?i)data-redis|jedis|lettuce`)},
	}
)

// BuildInfo is what the build file says.
type BuildInfo struct {
	Tool        string // "maven" or "gradle"
	JavaWants   int    // 0 when unknown
	SpringBoot  bool
	MultiModule bool
	DepKinds    map[string]string
}

// ParseBuild reads a pom.xml or Gradle build script.
func ParseBuild(tool, text string) BuildInfo {
	b := BuildInfo{Tool: tool, DepKinds: map[string]string{}}
	res, bootRe := gradleVersionRes, gradleBootRe
	if tool == "maven" {
		res, bootRe = mavenVersionRes, mavenBootRe
		b.MultiModule = modulesRe.MatchString(text)
	}
	for _, re := range res {
		if m := re.FindStringSubmatch(text); m != nil {
			if v := majorOf(m[1]); v > 0 {
				b.JavaWants = v
				break
			}
		}
	}
	b.SpringBoot = bootRe.MatchString(text)
	for _, d := range depKindRes {
		if m := d.re.FindString(text); m != "" {
			b.DepKinds[d.kind] = strings.ToLower(m)
		}
	}
	return b
}

// ParseProperties reads a Java .properties file into ordered key/value pairs.
func ParseProperties(text string) [][2]string {
	var out [][2]string
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		i := strings.IndexAny(line, "=:")
		if i < 0 {
			continue
		}
		out = append(out, [2]string{strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:])})
	}
	return out
}

// FlattenYAML turns the first document of a simple Spring application.yml into dotted
// keys. It handles nested maps only; lists, anchors and multi-line values are skipped.
func FlattenYAML(text string) [][2]string {
	type level struct {
		indent int
		key    string
	}
	var stack []level
	var out [][2]string
	sc := bufio.NewScanner(strings.NewReader(text))
	started := false
	for sc.Scan() {
		raw := strings.TrimRight(sc.Text(), " \t\r")
		trim := strings.TrimSpace(raw)
		if trim == "---" {
			if started {
				break
			}
			continue
		}
		if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, "- ") {
			continue
		}
		started = true
		if i := strings.Index(trim, " #"); i >= 0 {
			trim = strings.TrimSpace(trim[:i])
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		var key, val string
		if i := strings.Index(trim, ": "); i >= 0 {
			key, val = trim[:i], strings.TrimSpace(trim[i+2:])
		} else if strings.HasSuffix(trim, ":") {
			key = strings.TrimSuffix(trim, ":")
		} else {
			continue
		}
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if val == "" {
			stack = append(stack, level{indent, key})
			continue
		}
		parts := make([]string, 0, len(stack)+1)
		for _, l := range stack {
			parts = append(parts, l.key)
		}
		out = append(out, [2]string{strings.Join(append(parts, key), "."), val})
	}
	return out
}

// resolve turns "${DB_URL:jdbc:...}" into its default, and "${SECRET}" into an empty value
// plus the name of the environment variable it reads.
func resolve(v string) (val, envName string) {
	t := strings.TrimSpace(v)
	if !strings.HasPrefix(t, "${") || !strings.HasSuffix(t, "}") || strings.Count(t, "${") != 1 {
		return v, ""
	}
	name, def, hasDef := strings.Cut(t[2:len(t)-1], ":")
	if hasDef {
		return def, ""
	}
	return "", name
}

// LoadConfig reads src/main/resources/application.(properties|yml|yaml).
func LoadConfig(dir string) []project.EnvVar {
	res := filepath.Join(dir, "src", "main", "resources")
	var pairs [][2]string
	for _, name := range []string{"application.properties", "application.yml", "application.yaml"} {
		b, err := os.ReadFile(filepath.Join(res, name))
		if err != nil {
			continue
		}
		if strings.HasSuffix(name, ".properties") {
			pairs = append(pairs, ParseProperties(string(b))...)
		} else {
			pairs = append(pairs, FlattenYAML(string(b))...)
		}
	}
	var out []project.EnvVar
	for _, kv := range pairs {
		val, env := resolve(kv[1])
		v := project.EnvVar{Key: kv[0], Value: val}
		switch {
		case env != "":
			v.Kind, v.Reason = "blocking", "read from the environment variable "+env+", which has no default, so set it before starting"
		default:
			v.Kind, v.Reason = envfile.Classify(kv[0], val)
		}
		out = append(out, v)
	}
	return out
}

// Analyze reads one Maven or Gradle project.
func Analyze(root, dir string, installed int) (*project.JavaProject, error) {
	tool, file := "", ""
	switch {
	case fsx.FileExists(filepath.Join(dir, "pom.xml")):
		tool, file = "maven", "pom.xml"
	case fsx.FileExists(filepath.Join(dir, "build.gradle.kts")):
		tool, file = "gradle", "build.gradle.kts"
	case fsx.FileExists(filepath.Join(dir, "build.gradle")):
		tool, file = "gradle", "build.gradle"
	default:
		return nil, fmt.Errorf("no pom.xml or build.gradle found")
	}
	b, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return nil, err
	}
	info := ParseBuild(tool, string(b))
	j := &project.JavaProject{
		Dir: dir, Rel: fsx.Rel(root, dir), Build: tool, SpringBoot: info.SpringBoot,
		JavaWanted: info.JavaWants, JavaInstalled: installed,
		DepKinds: info.DepKinds, Config: LoadConfig(dir),
	}
	j.Wrapper = fsx.FileExists(filepath.Join(dir, wrapperFile(tool)))
	if info.SpringBoot {
		j.Port = "8080"
	}
	for _, v := range j.Config {
		if v.Key == "server.port" && v.Value != "" {
			j.Port, j.PortFromConfig = v.Value, true
		}
	}

	switch {
	case installed == 0:
		want := "a JDK"
		if j.JavaWanted > 0 {
			want = "JDK " + strconv.Itoa(j.JavaWanted)
		}
		j.Warnings = append(j.Warnings, "Java was not found in PATH. Install "+want+" (for example from https://adoptium.net/)")
	case j.JavaWanted > 0 && installed < j.JavaWanted:
		j.Warnings = append(j.Warnings, fmt.Sprintf("the project is set up for Java %d but Java %d is installed. Install JDK %d (for example from https://adoptium.net/)", j.JavaWanted, installed, j.JavaWanted))
	}
	if !j.Wrapper {
		tool := "mvn"
		if j.Build == "gradle" {
			tool = "gradle"
		}
		if _, err := exec.LookPath(tool); err != nil {
			j.Warnings = append(j.Warnings, fmt.Sprintf("no %s wrapper in the project and %s is not in PATH. Install %s", wrapperName(j.Build), tool, strings.ToUpper(tool[:1])+tool[1:]))
		}
	}
	if info.MultiModule {
		j.Warnings = append(j.Warnings, "this is a multi-module Maven build. gclone builds the root; to start one module use `-pl <module>` (see the README)")
	}
	return j, nil
}

func wrapperName(tool string) string {
	if tool == "maven" {
		return "mvnw"
	}
	return "gradlew"
}

func wrapperFile(tool string) string {
	if runtime.GOOS == "windows" {
		if tool == "maven" {
			return "mvnw.cmd"
		}
		return "gradlew.bat"
	}
	return wrapperName(tool)
}

// Argv returns the real command to run for goals, and Display the form shown to the user.
func Argv(j *project.JavaProject, goals ...string) []string {
	switch {
	case j.Wrapper && runtime.GOOS == "windows":
		return append([]string{filepath.Join(j.Dir, wrapperFile(j.Build))}, goals...)
	case j.Wrapper:
		return append([]string{"sh", wrapperName(j.Build)}, goals...)
	case j.Build == "maven":
		return append([]string{"mvn"}, goals...)
	default:
		return append([]string{"gradle"}, goals...)
	}
}

// Display shows the command the way a person would type it.
func Display(j *project.JavaProject, goals ...string) string {
	name := "mvn"
	if j.Build == "gradle" {
		name = "gradle"
	}
	if j.Wrapper {
		name = "./" + wrapperName(j.Build)
	}
	return strings.Join(append([]string{name}, goals...), " ")
}

// Goals for each action.
func DependencyGoals(j *project.JavaProject) []string {
	if j.Build == "maven" {
		return []string{"-B", "-q", "dependency:go-offline"}
	}
	return []string{"--console=plain", "classes"}
}

func BuildGoals(j *project.JavaProject) []string {
	if j.Build == "maven" {
		return []string{"-B", "-q", "-DskipTests", "package"}
	}
	return []string{"--console=plain", "build", "-x", "test"}
}

// TestGoals is the same for Maven and Gradle.
func TestGoals() []string { return []string{"test"} }

func RunGoals(j *project.JavaProject) []string {
	if j.Build == "maven" {
		return []string{"spring-boot:run"}
	}
	return []string{"--console=plain", "bootRun"}
}

// Runnables describes how to build and start each Java project for verification.
func Runnables(p *project.Project, noBuild bool) []project.Runnable {
	var out []project.Runnable
	for _, j := range p.JavaProjects {
		r := project.Runnable{Label: j.Rel + " (Java)", Dir: j.Dir}
		if !noBuild {
			r.Build = Argv(j, BuildGoals(j)...)
		}
		if j.SpringBoot {
			r.Start = Argv(j, RunGoals(j)...)
			if n, err := strconv.Atoi(j.Port); err == nil && n > 0 {
				r.Port = n
				r.PortFrom = "Spring Boot default"
				if j.PortFromConfig {
					r.PortFrom = "server.port in application config"
				}
			}
		} else {
			r.NoStart = "Java project without Spring Boot: built, not started (see the guide)"
		}
		for _, v := range j.Config {
			if v.Kind == "blocking" {
				r.Blocking = append(r.Blocking, v.Key)
			}
		}
		sort.Strings(r.Blocking)
		out = append(out, r)
	}
	return out
}
