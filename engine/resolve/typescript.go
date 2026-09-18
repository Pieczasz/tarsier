package resolve

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	tsFromRE  = regexp.MustCompile(`(?m)^\s*import\s+(?:type\s+)?(?:(\w+)|(?:\* as (\w+))|(?:\{[^}]+\}))\s+from\s+['"]([^'"]+)['"]`)
	tsSideRE  = regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`)
	tsConstRE = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(\[[^\]]*\]|['"][^'"]*['"])`)
	tsStrRE   = regexp.MustCompile(`['"]([^'"]+)['"]`)
	tsArrRE   = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*\[`)
)

func analyzeTS(f *File, src string) {
	collectTSImports(f, src)
	collectTSOneLineConsts(f, src)
	// ponytail: first "]" ends the array; nested brackets / strings with ] truncate.
	collectBracketArray(f, src, tsArrRE, tsStrRE)
}

func collectTSImports(f *File, src string) {
	for _, m := range tsFromRE.FindAllStringSubmatch(src, -1) {
		path := m[3]
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			// named-only import: use basename as a weak key for HasImport
			f.Imports[filepath.Base(path)] = path
			continue
		}
		f.Imports[name] = path
	}
	for _, m := range tsSideRE.FindAllStringSubmatch(src, -1) {
		path := m[1]
		f.Imports[filepath.Base(path)] = path
	}
}

func collectTSOneLineConsts(f *File, src string) {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := tsConstRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, rhs := m[1], m[2]
		if strings.HasPrefix(strings.TrimSpace(rhs), "[") {
			continue // multi-line path owns arrays (incl. one-liners below)
		}
		for _, sm := range tsStrRE.FindAllStringSubmatchIndex(rhs, -1) {
			f.Strings[name] = append(f.Strings[name], Lit{Value: rhs[sm[2]:sm[3]], Line: i + 1})
		}
	}
}
