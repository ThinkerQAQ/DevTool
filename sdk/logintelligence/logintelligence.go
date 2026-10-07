package logintelligence

const (
	ServiceName   = "log-analysis"
	MethodAnalyze = "analyze"
)

type AnalyzeRequest struct {
	Root  string `json:"root"`
	Path  string `json:"path"`
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type Summary struct {
	Lines     int64 `json:"lines"`
	Bytes     int64 `json:"bytes"`
	Matched   int   `json:"matched"`
	Errors    int   `json:"errors"`
	Warnings  int   `json:"warnings"`
	Truncated bool  `json:"truncated,omitempty"`
}

type Pattern struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	Sample  string `json:"sample,omitempty"`
}

type Evidence struct {
	Line  int    `json:"line"`
	Level string `json:"level,omitempty"`
	Text  string `json:"text"`
}

type AnalyzeResponse struct {
	Provider string         `json:"provider"`
	Source   string         `json:"source"`
	Format   string         `json:"format"`
	Summary  Summary        `json:"summary"`
	Levels   map[string]int `json:"levels,omitempty"`
	Patterns []Pattern      `json:"patterns,omitempty"`
	Matches  []Evidence     `json:"matches,omitempty"`
	Evidence []Evidence     `json:"evidence,omitempty"`
	LnavUsed bool           `json:"lnav_used"`
}
