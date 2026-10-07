package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/thinkerqaq/devtool/internal/buildinfo"
)

func TestRunVersionJSON(t *testing.T) {
	oldVersion, oldCommit, oldDirty := buildinfo.Version, buildinfo.Commit, buildinfo.Dirty
	t.Cleanup(func() {
		buildinfo.Version, buildinfo.Commit, buildinfo.Dirty = oldVersion, oldCommit, oldDirty
	})
	buildinfo.Version = "1.2.3-test"
	buildinfo.Commit = "abc123"
	buildinfo.Dirty = "true"

	var out bytes.Buffer
	if err := runVersion([]string{"--json"}, &out); err != nil {
		t.Fatalf("runVersion() error = %v", err)
	}
	var got buildinfo.Info
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode version JSON: %v", err)
	}
	if got.Version != "1.2.3-test" || got.Commit != "abc123" || !got.Dirty {
		t.Fatalf("version info = %+v", got)
	}
}
