// Package adapter defines what an ecosystem (Node.js, Go, Java, and later others) has to
// provide so gclone can analyze, plan, verify and document it.
//
// The shared flow stays in the analyze, plan, verify and guide packages. They loop over the
// adapters and never need to know which ecosystems exist. To add one, implement Adapter in
// internal/adapter/<name> and list it in internal/adapters.
package adapter

import "github.com/rauni5/gclone/internal/project"

// Adapter knows one ecosystem. Every method must be safe to call when the repository holds
// none of its projects: it returns nothing.
type Adapter interface {
	// Name is the ecosystem's name, for messages ("Node.js", "Go", "Java").
	Name() string

	// Analyze finds this ecosystem's projects under root and records them on p. It reads
	// files only and changes nothing on the machine.
	Analyze(p *project.Project, root string)

	// Units is how many projects of this ecosystem were found.
	Units(p *project.Project) int
	// Markers are build files this adapter handles. They are not reported as an
	// unsupported stack when Units is above zero.
	Markers() []string

	// Detected returns one line per project for the "Detected:" list.
	Detected(p *project.Project) []string
	// Warnings returns problems found during analysis, already prefixed with the folder.
	Warnings(p *project.Project) []string

	// Prepare returns steps that run before services start (for example creating .env).
	Prepare(p *project.Project, o project.Options) []project.Planned
	// Install returns the steps that fetch dependencies.
	Install(p *project.Project, o project.Options) []project.Planned

	// Runnables says how to build and start each project, for gclone verify.
	Runnables(p *project.Project, noBuild bool) []project.Runnable
	// Next returns the commands to show under "Next:".
	Next(p *project.Project) []string

	// Guide returns the "run it" sections for the setup guide, as Markdown.
	Guide(p *project.Project) string
	// GuideConfig returns the "what you may need to change" sections, as Markdown.
	GuideConfig(p *project.Project) []string
}
