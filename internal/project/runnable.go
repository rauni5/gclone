package project

// Runnable describes how one part of a repository is built and started. This is
// the seam for other ecosystems: an adapter for Java or Flutter only has to
// produce Runnables, and verification works the same way.
type Runnable struct {
	Label       string
	Dir         string
	Build       []string // nil when there is nothing to build
	Start       []string // nil when there is no way to start it
	NoStart     string   // why nothing is started, shown instead of a warning
	Port        int      // 0 when unknown
	PortFrom    string
	ExampleName string // env example file, empty when none
	HasEnv      bool
	Blocking    []string // variables still empty or placeholder in the real .env
	Missing     []string // read by the code but defined nowhere
}
