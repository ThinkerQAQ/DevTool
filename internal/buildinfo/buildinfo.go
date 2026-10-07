package buildinfo

import "strings"

var (
	Version = "dev"
	Commit  = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Current() Info {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "dev"
	}
	commit := strings.TrimSpace(Commit)
	if commit == "" {
		commit = "unknown"
	}
	return Info{Version: version, Commit: commit}
}
