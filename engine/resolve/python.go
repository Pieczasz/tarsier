package resolve

import (
	"regexp"
	"strings"
)

var (
	pyImportRE = regexp.MustCompile(`(?m)^\s*(?:from\s+([\w.]+)\s+import\s+([\w* ,]+)|import\s+([\w.]+)(?:\s+as\s+(\w+))?)`)
	pyFuncRE   = regexp.MustCompile(`(?m)^\s*def\s+\w+\s*\(([^)]*)\)`)
	pyParamRE  = regexp.MustCompile(`(\w+)\s*:\s*([\w.]+)`)
	pyAssignRE = regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*(\[[^\]]*\]|['"][^'"]*['"])`)
)

func analyzePython(f *File, src string) {
	collectPyImports(f, src)
	collectPyParams(f, src)
	collectPyAssigns(f, src)
}

func collectPyImports(f *File, src string) {
	for _, m := range pyImportRE.FindAllStringSubmatch(src, -1) {
		switch {
		case m[1] != "":
			mod := m[1]
			for name := range strings.SplitSeq(m[2], ",") {
				name = strings.TrimSpace(name)
				if name == "" || name == "*" {
					continue
				}
				if before, after, ok := strings.Cut(name, " as "); ok {
					f.Imports[strings.TrimSpace(after)] = mod + "." + strings.TrimSpace(before)
				} else {
					f.Imports[name] = mod + "." + name
				}
			}
		case m[3] != "":
			mod := m[3]
			name := mod
			if i := strings.LastIndex(mod, "."); i >= 0 {
				name = mod[i+1:]
			}
			if m[4] != "" {
				name = m[4]
			}
			f.Imports[name] = mod
		}
	}
}

func collectPyParams(f *File, src string) {
	for _, m := range pyFuncRE.FindAllStringSubmatch(src, -1) {
		for _, p := range pyParamRE.FindAllStringSubmatch(m[1], -1) {
			f.Params[p[1]] = p[2]
		}
	}
}

func collectPyAssigns(f *File, src string) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := pyAssignRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, rhs := m[1], m[2]
		for _, sm := range tsStrRE.FindAllStringSubmatchIndex(rhs, -1) {
			f.Strings[name] = append(f.Strings[name], Lit{Value: rhs[sm[2]:sm[3]], Line: i + 1})
		}
	}
}
