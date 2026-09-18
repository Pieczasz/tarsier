package resolve

import (
	"regexp"
	"strconv"
	"strings"
)

// collectBracketArray finds name = [ ... ] bodies and records quoted strings.
// The first "]" after the header ends the body (thin-resolver limit).
func collectBracketArray(f *File, src string, header, strRE *regexp.Regexp) {
	for _, loc := range header.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		start := loc[1]
		end := strings.Index(src[start:], "]")
		if end < 0 {
			continue
		}
		body := src[start : start+end]
		baseLine := strings.Count(src[:start], "\n") + 1
		for _, sm := range strRE.FindAllStringSubmatchIndex(body, -1) {
			line := baseLine + strings.Count(body[:sm[0]], "\n")
			f.Strings[name] = append(f.Strings[name], Lit{Value: body[sm[2]:sm[3]], Line: line})
		}
	}
}

// collectBraceArray finds name = { ... } bodies (Java String[]) and records
// double-quoted strings. The first "}" after the header ends the body.
func collectBraceArray(f *File, src string, header, strRE *regexp.Regexp) {
	for _, loc := range header.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		start := loc[1]
		end := strings.Index(src[start:], "}")
		if end < 0 {
			continue
		}
		body := src[start : start+end]
		baseLine := strings.Count(src[:start], "\n") + 1
		for _, sm := range strRE.FindAllStringIndex(body, -1) {
			raw := body[sm[0]:sm[1]]
			v, err := strconv.Unquote(raw)
			if err != nil {
				v = strings.Trim(raw, `"`)
			}
			line := baseLine + strings.Count(body[:sm[0]], "\n")
			f.Strings[name] = append(f.Strings[name], Lit{Value: v, Line: line})
		}
	}
}
