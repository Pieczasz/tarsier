package pattern

// match is one ast-grep `--json=stream` object. Only the fields we consume are
// declared; ast-grep emits considerably more (labels, charCount, byte offsets).
type match struct {
	Text     string         `json:"text"`
	Range    matchRange     `json:"range"`
	File     string         `json:"file"`
	Language string         `json:"language"`
	RuleID   string         `json:"ruleId"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Note     string         `json:"note"`
	Metadata map[string]any `json:"metadata"`
}

type matchRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

// position is zero-based in ast-grep's output; finding.Location.Line is not.
type position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

func (m *match) meta(key string) string {
	if s, ok := m.Metadata[key].(string); ok {
		return s
	}
	return ""
}
