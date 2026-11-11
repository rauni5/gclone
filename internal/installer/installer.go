package installer

import (
	"github.com/rauni5/gclone/internal/proc"
)

type Option struct {
	Manager string
	Cmds    [][]string
	Note    string
}

func (n *Option) Run() error {
	for _, c := range n.Cmds {
		if err := proc.RunStream("", true, c[0], c[1:]...); err != nil {
			return err
		}
	}
	return nil
}
