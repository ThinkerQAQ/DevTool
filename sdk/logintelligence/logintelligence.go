package logintelligence

const (
	ServiceName   = "log-analysis"
	MethodAnalyze = "analyze"

	SourceFile     = "file"
	SourceJournald = "journald"
)

type Source struct {
	Kind       string `json:"kind,omitempty"`
	Path       string `json:"path,omitempty"`
	Unit       string `json:"unit,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Since      string `json:"since,omitempty"`
	Until      string `json:"until,omitempty"`
	MaxEntries int    `json:"max_entries,omitempty"`
}

type AnalyzeRequest struct {
	Root   string  `json:"root"`
	Path   string  `json:"path,omitempty"`
	Source *Source `json:"source,omitempty"`
	Query  string  `json:"query,omitempty"`
	Limit  int     `json:"limit,omitempty"`
}

type Summary struct {
	Lines            int64 `json:"lines"`
	Bytes            int64 `json:"bytes"`
	Matched          int   `json:"matched"`
	Errors           int   `json:"errors"`
	Warnings         int   `json:"warnings"`
	EmbeddedErrors   int   `json:"embedded_errors,omitempty"`
	EmbeddedWarnings int   `json:"embedded_warnings,omitempty"`
	Truncated        bool  `json:"truncated,omitempty"`
}

type Pattern struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	Sample  string `json:"sample,omitempty"`
}

type Evidence struct {
	Line          int    `json:"line"`
	Time          string `json:"time,omitempty"`
	Source        string `json:"source,omitempty"`
	Level         string `json:"level,omitempty"`
	EmbeddedLevel string `json:"embedded_level,omitempty"`
	Text          string `json:"text"`
}

type AnalyzeResponse struct {
	Provider       string         `json:"provider"`
	Source         string         `json:"source"`
	SourceKind     string         `json:"source_kind,omitempty"`
	Format         string         `json:"format"`
	Summary        Summary        `json:"summary"`
	Levels         map[string]int `json:"levels,omitempty"`
	EmbeddedLevels map[string]int `json:"embedded_levels,omitempty"`
	Patterns       []Pattern      `json:"patterns,omitempty"`
	Matches        []Evidence     `json:"matches,omitempty"`
	Evidence       []Evidence     `json:"evidence,omitempty"`
	LnavUsed       bool           `json:"lnav_used"`
}
