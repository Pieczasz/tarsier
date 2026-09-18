package resolve

import (
	"regexp"
	"strings"
)

var (
	javaImportRE = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+)\s*;`)
	javaArrRE    = regexp.MustCompile(`(?m)^\s*(?:private|public|protected)?\s*(?:static\s+)?(?:final\s+)?String\s*\[\s*\]\s+(\w+)\s*=\s*\{`)
	javaStrRE    = regexp.MustCompile(`"([^"\\]|\\.)*"`)
)

func analyzeJava(f *File, src string) {
	for _, m := range javaImportRE.FindAllStringSubmatch(src, -1) {
		path := m[1]
		parts := strings.Split(path, ".")
		f.Imports[parts[len(parts)-1]] = path
	}
	// ponytail: first "}" ends the array; nested braces / strings with } truncate.
	collectBraceArray(f, src, javaArrRE, javaStrRE)
}
