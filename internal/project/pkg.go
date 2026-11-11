package project

func PortFor(pkg *Pkg) string {
	if pkg.Port != "" {
		return pkg.Port
	}
	for _, v := range pkg.EnvVars {
		if v.Key == "PORT" && v.Value != "" {
			return v.Value + " (from PORT)"
		}
	}
	return ""
}

func DevScript(pkg *Pkg) string {
	for _, s := range []string{"dev", "start", "serve", "develop"} {
		if _, ok := pkg.Scripts[s]; ok {
			return s
		}
	}
	return ""
}
