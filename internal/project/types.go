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
// FlutterProject is one Flutter app (a pubspec.yaml that depends on the Flutter SDK).
type FlutterProject struct {
	Dir              string
	Rel              string
	Name             string
	DartWanted       string // lower bound of environment.sdk, "" when unknown
	FlutterWanted    string // lower bound of environment.flutter, "" when unknown
	DartInstalled    string // "" when Flutter is not installed
	FlutterInstalled string
	Platforms        []string // android, ios, web, windows, linux, macos
	Firebase         bool
	MissingFirebase  []string // Firebase config files that are not in the folder
	EnvAsset         bool     // pubspec lists .env as an asset
	EnvExample       string
	EnvCreated       bool
	EnvVars          []EnvVar
	BackendPorts     []string // ports of localhost / 10.0.2.2 addresses found in lib/
	HasTests         bool
	DartDefines      []string // names (never values) passed with --dart-define, from launch.json and the README
	DartDefineFiles  []string // files named by --dart-define-from-file, relative to the app
	CodeGen          []string // code generation packages in pubspec.yaml (build_runner, freezed, ...)
	Warnings         []string
}

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

// PyProject is one Python project: a folder with requirements.txt, pyproject.toml, a
// Pipfile, setup.py or manage.py.
type PyProject struct {
	Dir               string
	Rel               string
	Name              string
	Manager           string   // pip, poetry, uv or pipenv
	Requirements      string   // requirements file used by pip, empty when installing from pyproject or setup.py
	ExtraRequirements []string // other requirements files found, e.g. requirements-dev.txt; reported but not installed
	Framework         string   // Django, FastAPI, Flask, Streamlit or empty
	Port              string
	Start             []string // arguments after the Python interpreter, nil when unknown
	StartNote         string   // why Start is empty
	PyWanted          string   // lowest Python version the project asks for, "3.10"
	PyUpper           string   // Python version it does not support from, "3.13"
	PySource          string   // where PyWanted came from
	PyInstalled       string   // installed Python, empty when none was found
	HasVenv           bool
	HasTests          bool
	Pytest            bool
	EnvExample        string
	EnvCreated        bool
	EnvVars           []EnvVar
	MissingVars       []string
	DepKinds          map[string]string
	Warnings          []string
}

// Project is the analysed repository.
type Project struct {
	URL             string
	Dir             string
	Commit          string
	DockerState     string // "", "ready", "stopped", "missing" or "denied"
	Executed        bool
	Pkgs            []*Pkg
	GoModules       []*GoModule
	JavaProjects    []*JavaProject
	PyProjects      []*PyProject
	FlutterProjects []*FlutterProject
	Composes        []*Compose
	Needs           []*Need
	Companions      []*Companion
	Verify          *Report
	OtherStacks     []string
	Warnings        []string
	Steps           []Step
}
