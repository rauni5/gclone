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

// Project is the analysed repository.
type Project struct {
	URL         string
	Dir         string
	Commit      string
	DockerState string // "", "ready", "stopped", "missing" or "denied"
	Executed    bool
	Pkgs        []*Pkg
	Composes    []*Compose
	Needs       []*Need
	Companions  []*Companion
	Verify      *Report
	OtherStacks []string
	Warnings    []string
	Steps       []Step
}
