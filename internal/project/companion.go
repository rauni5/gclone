package project

// apiRef is an env variable that points at a backend on localhost.
type ApiRef struct {
	Pkg, Key, Value string
	Port            int
}

// Companion is another repository this project needs in order to run.
type Companion struct {
	Name    string
	URL     string // empty when unknown
	Reason  string
	Result  string
	Dir     string
	Refs    []ApiRef
	Project *Project
}
