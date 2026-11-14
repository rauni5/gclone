package project

// EnvVar is one variable parsed from an env example file.
type EnvVar struct {
	Key     string
	Value   string
	Comment string
	Kind    string // "blocking", "review" or "ok"
	Reason  string
}

// Step records the result of one setup action.
type Step struct {
	Where  string
	Name   string
	Cmd    string
	Status string // "ok" or "failed"
	Note   string
	Hint   string // plain-language explanation of a failure, when one is known
}

// Pkg is one Node.js package directory inside the cloned repository.
type Pkg struct {
	Dir              string // absolute path
	Rel              string // path relative to the repo root, "." for the root
	Name             string
	PM               string // npm, pnpm, yarn or bun
	HasLock          bool
	NodeWanted       string
	NodeSource       string
	Scripts          map[string]string
	Framework        string
	Port             string
	LifecycleScripts []string
	EnvExample       string // file name, empty when none
	EnvCreated       bool
	EnvVars          []EnvVar
	MissingVars      []string // referenced in code but absent from the example file
	DBHints          []string
	DepKinds         map[string]string // service kind -> dependency that implies it
	LocalDeps        map[string]string // dependency name -> relative path outside the package (file:../x)
	Prisma           bool
	Warnings         []string
}

// Compose is a docker compose file found in the repository.
type Compose struct {
	Path        string
	Rel         string
	AllServices []string
	DBServices  []string
	Images      map[string]string
	Outside     []string // build contexts that point outside the repository (../backend)
	Started     bool
}

// GoModule is one Go module (a directory with a go.mod) inside the repository.
type GoModule struct {
	Dir       string
	Rel       string
	Module    string // module path from go.mod
	GoWanted  string // go directive, for example "1.24"
	Toolchain string // toolchain directive, empty when absent
	Installed string // installed Go version, empty when Go is not installed
	HasMain   bool   // main.go at the module root
	BinName   string // name used when building the binary
	Warnings  []string
}

// JavaProject is one Maven or Gradle project, with Spring Boot details when present.
type JavaProject struct {
	Dir            string
	Rel            string
	Build          string // "maven" or "gradle"
	Wrapper        bool   // mvnw or gradlew present
	SpringBoot     bool
	JavaWanted     int // major version the build file asks for, 0 when unknown
	JavaInstalled  int // 0 when Java is not installed
	Port           string
	PortFromConfig bool
	Config         []EnvVar          // application.properties or .yml, flattened
	DepKinds       map[string]string // service kind -> dependency that implies it
	Warnings       []string
}

// Project is the analysed repository.
type Project struct {
	URL          string
	Dir          string
	Commit       string
	DockerState  string // "", "ready", "stopped", "missing" or "denied"
	Executed     bool
	Pkgs         []*Pkg
	GoModules    []*GoModule
	JavaProjects []*JavaProject
	Composes     []*Compose
	Needs        []*Need
	Companions   []*Companion
	Verify       *Report
	OtherStacks  []string
	Warnings     []string
	Steps        []Step
}
