// Package cli implements replaceable, opt-in diagram rendering through installed
// executables. It never downloads tools or invokes a shell.
package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
	extension "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "diagram.render.cli"
const maxSource = 256 << 10
const maxSVG = 8 << 20

type Extension struct {
	mermaid  string
	plantuml string
	timeout  time.Duration
}

func New() *Extension {
	return &Extension{mermaid: "mmdc", plantuml: "plantuml", timeout: 30 * time.Second}
}
func (e *Extension) Descriptor() extension.Descriptor {
	return extension.Descriptor{ID: ExtensionID, Kind: extension.KindDocumentIntelligence, Provides: []string{diagram.RenderServiceName}}
}
func (e *Extension) Configure(s map[string]any) error {
	if x, ok := s["mermaid_bin"]; ok {
		v, yes := x.(string)
		if !yes || strings.TrimSpace(v) == "" {
			return fmt.Errorf("mermaid_bin must be non-empty string")
		}
		e.mermaid = v
	}
	if x, ok := s["plantuml_bin"]; ok {
		v, yes := x.(string)
		if !yes || strings.TrimSpace(v) == "" {
			return fmt.Errorf("plantuml_bin must be non-empty string")
		}
		e.plantuml = v
	}
	if x, ok := s["timeout_seconds"]; ok {
		f, yes := x.(float64)
		if !yes {
			if v, yes2 := x.(int64); yes2 {
				f = float64(v)
				yes = true
			}
		}
		if !yes || f < 1 || f > 300 || f != float64(int(f)) {
			return fmt.Errorf("timeout_seconds must be 1..300")
		}
		e.timeout = time.Duration(int(f)) * time.Second
	}
	return nil
}
func (e *Extension) Register(r extension.Registrar) error {
	return r.ProvideService(diagram.RenderServiceName, ExtensionID, service.Func(e.Invoke))
}
func (e *Extension) Invoke(ctx context.Context, method string, data json.RawMessage) (json.RawMessage, error) {
	if method != diagram.RenderMethod {
		return nil, fmt.Errorf("%s does not support %q", ExtensionID, method)
	}
	var in diagram.RenderRequest
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, err
	}
	result := e.render(ctx, in)
	return json.Marshal(result)
}
func (e *Extension) render(ctx context.Context, in diagram.RenderRequest) diagram.RenderResult {
	lang := strings.ToLower(strings.TrimSpace(in.Language))
	bin := e.mermaid
	switch lang {
	case "mermaid":
	case "plantuml":
		bin = e.plantuml
	default:
		return diagram.RenderResult{Status: "unsupported", Diagnostic: "unsupported diagram language: " + lang}
	}
	if len(in.Source) == 0 || len(in.Source) > maxSource {
		return diagram.RenderResult{Status: "failed", Diagnostic: "diagram source must contain 1..262144 bytes"}
	}
	executable, err := exec.LookPath(bin)
	if err != nil {
		return diagram.RenderResult{Status: "unavailable", Renderer: bin, Diagnostic: fmt.Sprintf("renderer %q is not installed or not on PATH; configure its executable; DevTool does not install dependencies", bin)}
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
	}
	outDir := filepath.Join(cacheDir, "devtool", "diagrams")
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
	}
	fingerprint := sha256.Sum256([]byte(lang + "\x00" + in.Source))
	name := hex.EncodeToString(fingerprint[:]) + ".svg"
	final := filepath.Join(outDir, name)
	tempDir, err := os.MkdirTemp(outDir, "render-")
	if err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
	}
	defer os.RemoveAll(tempDir)
	sourceFile := filepath.Join(tempDir, "diagram.mmd")
	tempSVG := filepath.Join(tempDir, "diagram.svg")
	if err := os.WriteFile(sourceFile, []byte(in.Source), 0600); err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
	}
	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	var cmd *exec.Cmd
	if lang == "mermaid" {
		cmd = exec.CommandContext(runCtx, executable, "-i", sourceFile, "-o", tempSVG)
	} else {
		cmd = exec.CommandContext(runCtx, executable, "-tsvg", "-pipe")
		cmd.Stdin = strings.NewReader(in.Source)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		diagnostic := strings.TrimSpace(stderr.String() + "\n" + stdout.String())
		if runCtx.Err() != nil {
			diagnostic = runCtx.Err().Error()
		}
		if diagnostic == "" {
			diagnostic = err.Error()
		}
		if len(diagnostic) > 1600 {
			diagnostic = diagnostic[:1600]
		}
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: diagnostic}
	}
	if lang == "plantuml" {
		if err := os.WriteFile(tempSVG, stdout.Bytes(), 0600); err != nil {
			return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
		}
	}
	data, err := os.ReadFile(tempSVG)
	if err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: "renderer did not produce an SVG: " + err.Error()}
	}
	if len(data) == 0 || len(data) > maxSVG || !bytes.Contains(data, []byte("<svg")) {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: "renderer output is not a bounded SVG artifact"}
	}
	if err := os.WriteFile(final, data, 0600); err != nil {
		return diagram.RenderResult{Status: "failed", Renderer: bin, Diagnostic: err.Error()}
	}
	return diagram.RenderResult{Status: "rendered", Renderer: bin, ArtifactPath: final}
}
