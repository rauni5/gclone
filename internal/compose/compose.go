package compose

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rauni5/gclone/internal/fsx"
	"github.com/rauni5/gclone/internal/project"
)

var composeNames = []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}

func FindComposes(root string, pkgs []*project.Pkg) []*project.Compose {
	dirs := []string{root}
	for _, p := range pkgs {
		if p.Dir != root {
			dirs = append(dirs, p.Dir)
		}
	}
	seen := map[string]bool{}
	var out []*project.Compose
	for _, d := range dirs {
		for _, n := range composeNames {
			path := filepath.Join(d, n)
			if fsx.FileExists(path) && !seen[path] {
				seen[path] = true
				if c := ParseCompose(root, path); c != nil {
					out = append(out, c)
				}
				break
			}
		}
	}
	return out
}

var ctxRe = regexp.MustCompile(`^(?:build|context):\s*["']?(\.\./[^"'\s#]*)`)

var svcRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+):\s*(#.*)?$`)

var imageRe = regexp.MustCompile(`^image:\s*["']?([^"'\s#]+)`)

var dbImage = regexp.MustCompile(`(?i)(^|/)(postgres|postgresql|mysql|mariadb|mongo|mongodb|redis|memcached|valkey|rabbitmq)(:|@|$)`)

// parseCompose is a small line-based reader. It only needs service names and
// image names, so it avoids a YAML dependency. Unusual layouts are skipped.
func ParseCompose(root, path string) *project.Compose {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	c := &project.Compose{Path: path, Rel: fsx.Rel(root, path), Images: map[string]string{}}
	inServices := false
	svcIndent := -1
	cur := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inServices = trim == "services:"
			svcIndent, cur = -1, ""
			continue
		}
		if !inServices {
			continue
		}
		if svcIndent == -1 {
			svcIndent = indent
		}
		if indent == svcIndent {
			if m := svcRe.FindStringSubmatch(trim); m != nil {
				cur = m[1]
				c.AllServices = append(c.AllServices, cur)
			} else {
				cur = ""
			}
			continue
		}
		if cur != "" {
			if m := imageRe.FindStringSubmatch(trim); m != nil {
				c.Images[cur] = m[1]
			}
			if m := ctxRe.FindStringSubmatch(trim); m != nil {
				c.Outside = append(c.Outside, m[1])
			}
		}
	}
	for _, s := range c.AllServices {
		if img := c.Images[s]; img != "" && dbImage.MatchString(img) {
			c.DBServices = append(c.DBServices, s)
		}
	}
	return c
}
