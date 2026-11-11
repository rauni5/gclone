package project

// Need is a database or service the project depends on.
type Need struct {
	Kind       string
	Name       string
	Port       int
	Reasons    []string
	User, Pass string
	DB         string
	Remote     bool
	RemoteHost string
	Result     string
	Container  string
}
