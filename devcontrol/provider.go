package devcontrol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thinkerqaq/devtool/core/contract"
	environmentcontract "github.com/thinkerqaq/devtool/sdk/environment"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/project"
)

type Provider struct{}

func (Provider) ExtensionDescriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       "project.devtool",
		Kind:     extensioncontract.KindProject,
		Requires: []string{environmentcontract.ServiceName},
	}
}

func (Provider) ProjectDescriptor() contract.ProjectDescriptor {
	return contract.ProjectDescriptor{
		Identity: contract.ProjectIdentity{Name: "DevTool"},
		Commands: []contract.CommandDescriptor{
			{ID: "activate", Title: "Activate", Description: "Atomically activate the verified DevTool candidate in the user PATH installation.", SideEffect: contract.SideEffectWrite},
			{ID: "build", Title: "Build", Description: "Build the next DevTool binary in the configured development environment.", SideEffect: contract.SideEffectWrite},
			{ID: "package", Title: "Package", Description: "Package a verified DevTool binary in the configured development environment.", SideEffect: contract.SideEffectWrite},
			{ID: "rollback", Title: "Rollback", Description: "Atomically restore the previously activated DevTool version.", SideEffect: contract.SideEffectWrite},
			{ID: "verify", Title: "Verify", Description: "Run DevTool self-host verification in the configured development environment.", SideEffect: contract.SideEffectWrite},
		},
		Resources: []contract.ResourceDescriptor{
			{ID: "environment", Title: "Environment", Description: "DevTool development environment."},
		},
		Views: []contract.ViewDescriptor{
			{
				ID:        "overview",
				Title:     "Overview",
				Resources: []string{"environment"},
				Actions: []contract.ActionDescriptor{
					{CommandID: "build", Label: "Build"},
					{CommandID: "verify", Label: "Verify"},
					{CommandID: "activate", Label: "Activate"},
					{CommandID: "rollback", Label: "Rollback"},
					{CommandID: "package", Label: "Package"},
				},
			},
		},
		Navigation: []contract.NavigationItem{
			{ID: "overview", Title: "Overview", ViewID: "overview"},
		},
	}
}

func (Provider) Execute(ctx project.Context, command string, _ map[string]any) error {
	workspace, err := os.Getwd()
	if err != nil {
		return err
	}

	switch command {
	case "activate":
		return activate(ctx, workspace)
	case "build":
		return build(ctx, workspace)
	case "verify":
		return verify(ctx, workspace)
	case "rollback":
		return rollback(ctx)
	case "package":
		return packageArtifact(ctx, workspace)
	default:
		return fmt.Errorf("unknown DevTool project command %q", command)
	}
}

func build(ctx project.Context, workspace string) error {
	metadata, err := loadBuildMetadata(workspace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "out"), 0o755); err != nil {
		return fmt.Errorf("create DevTool output directory: %w", err)
	}
	if err := removeVerificationManifest(workspace); err != nil {
		return err
	}
	if err := ctx.Emit("progress", fmt.Sprintf("Building DevTool %s (%s) N+1 in configured environment", metadata.Version, shortCommit(metadata.Commit))); err != nil {
		return err
	}
	if err := runEnvironment(ctx, workspace, environmentcontract.CommandRequest{
		Executable: "go",
		Args:       devtoolBuildArgs(".devtool/out/"+nextBinaryName(), metadata, false),
		Env:        targetEnv(),
	}); err != nil {
		return err
	}
	return ctx.Emit("result", "DevTool N+1 built in configured environment")
}

func verify(ctx project.Context, workspace string) error {
	metadata, err := loadBuildMetadata(workspace)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "out"), 0o755); err != nil {
		return fmt.Errorf("create DevTool output directory: %w", err)
	}
	if err := removeVerificationManifest(workspace); err != nil {
		return err
	}
	if err := ctx.Emit("progress", "Verifying DevTool self-hosting in configured environment"); err != nil {
		return err
	}

	steps := []environmentcontract.CommandRequest{
		{Executable: "go", Args: []string{"test", "./..."}, Env: selfHostTestEnv()},
		{Executable: "go", Args: []string{"-C", "devcontrol", "test", "./..."}, Env: selfHostTestEnv()},
		{
			Executable: "go",
			Args:       devtoolBuildArgs(".devtool/out/"+nextBinaryName(), metadata, false),
		},
		{
			Executable: ".devtool/out/" + nextBinaryName(),
			Args:       []string{"project", "inspect", "--json"},
		},
	}
	for _, step := range steps {
		if err := runEnvironment(ctx, workspace, step); err != nil {
			return err
		}
	}
	if err := verifyCandidateMetadata(ctx, workspace, metadata); err != nil {
		return err
	}
	if err := writeVerificationManifest(workspace, metadata); err != nil {
		return err
	}
	return ctx.Emit("result", "DevTool self-host verification passed in configured environment")
}

func packageArtifact(ctx project.Context, workspace string) error {
	metadata, err := loadBuildMetadata(workspace)
	if err != nil {
		return err
	}
	if err := verify(ctx, workspace); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".devtool", "artifacts"), 0o755); err != nil {
		return fmt.Errorf("create DevTool artifact directory: %w", err)
	}
	if err := ctx.Emit("progress", "Packaging DevTool in configured environment"); err != nil {
		return err
	}
	if err := runEnvironment(ctx, workspace, environmentcontract.CommandRequest{
		Executable: "go",
		Args:       devtoolBuildArgs(".devtool/artifacts/"+packageBinaryName(metadata.Version), metadata, true),
		Env:        targetEnv(),
	}); err != nil {
		return err
	}
	return ctx.Emit("result", "DevTool package exported from configured environment")
}

func runEnvironment(ctx project.Context, workspace string, request environmentcontract.CommandRequest) error {
	_, err := runEnvironmentResult(ctx, workspace, request)
	return err
}

func runEnvironmentResult(ctx project.Context, workspace string, request environmentcontract.CommandRequest) (environmentcontract.RunResult, error) {
	request.Root = workspace

	var result environmentcontract.RunResult
	if err := ctx.InvokeService(
		environmentcontract.ServiceName,
		environmentcontract.MethodRun,
		request,
		&result,
	); err != nil {
		return environmentcontract.RunResult{}, err
	}
	if result.ExitCode != 0 {
		message := strings.TrimSpace(result.Stderr)
		if message == "" {
			message = strings.TrimSpace(result.Stdout)
		}
		if message == "" {
			message = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		return result, fmt.Errorf("%s failed: %s", request.Executable, message)
	}
	return result, nil
}

func verifyCandidateMetadata(ctx project.Context, workspace string, metadata buildMetadata) error {
	result, err := runEnvironmentResult(ctx, workspace, environmentcontract.CommandRequest{
		Executable: ".devtool/out/" + nextBinaryName(),
		Args:       []string{"version", "--json"},
	})
	if err != nil {
		return err
	}
	var got struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Dirty   bool   `json:"dirty"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.Stdout)), &got); err != nil {
		return fmt.Errorf("decode DevTool candidate version: %w", err)
	}
	if got.Version != metadata.Version || got.Commit != metadata.Commit || got.Dirty != metadata.Dirty {
		return fmt.Errorf(
			"DevTool candidate metadata mismatch: got version=%s commit=%s dirty=%t, want version=%s commit=%s dirty=%t",
			got.Version, got.Commit, got.Dirty, metadata.Version, metadata.Commit, metadata.Dirty,
		)
	}
	return nil
}

func verificationManifestPath(workspace string) string {
	return filepath.Join(workspace, ".devtool", "out", nextBinaryName()+".verified.json")
}

func removeVerificationManifest(workspace string) error {
	path := verificationManifestPath(workspace)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale DevTool verification manifest: %w", err)
	}
	return nil
}

func writeVerificationManifest(workspace string, metadata buildMetadata) error {
	binaryPath := filepath.Join(workspace, ".devtool", "out", nextBinaryName())
	digest, err := sha256File(binaryPath)
	if err != nil {
		return err
	}
	manifest := verificationManifest{
		SchemaVersion: 1,
		Version:       metadata.Version,
		Commit:        metadata.Commit,
		Dirty:         metadata.Dirty,
		SHA256:        digest,
		Binary:        nextBinaryName(),
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	path := verificationManifestPath(workspace)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write DevTool verification manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("activate DevTool verification manifest: %w", err)
	}
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func selfHostTestEnv() []string {
	return []string{"DEVTOOL_PROFILES="}
}

func targetEnv() []string {
	return []string{
		"CGO_ENABLED=0",
		"GOOS=" + runtime.GOOS,
		"GOARCH=" + runtime.GOARCH,
	}
}

func nextBinaryName() string {
	if runtime.GOOS == "windows" {
		return "devtool-next.exe"
	}
	return "devtool-next"
}

type buildMetadata struct {
	Version string
	Commit  string
	Dirty   bool
}

type verificationManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	Dirty         bool   `json:"dirty"`
	SHA256        string `json:"sha256"`
	Binary        string `json:"binary"`
}

func loadBuildMetadata(workspace string) (buildMetadata, error) {
	rawVersion, err := os.ReadFile(filepath.Join(workspace, "VERSION"))
	if err != nil {
		return buildMetadata{}, fmt.Errorf("read DevTool VERSION: %w", err)
	}
	version := strings.TrimSpace(string(rawVersion))
	if version == "" {
		return buildMetadata{}, fmt.Errorf("DevTool VERSION is empty")
	}

	cmd := exec.Command("git", "-C", workspace, "rev-parse", "HEAD")
	rawCommit, err := cmd.CombinedOutput()
	if err != nil {
		return buildMetadata{}, fmt.Errorf("resolve DevTool source commit: %w: %s", err, strings.TrimSpace(string(rawCommit)))
	}
	commit := strings.TrimSpace(string(rawCommit))
	if commit == "" {
		return buildMetadata{}, fmt.Errorf("resolve DevTool source commit: empty commit")
	}
	dirtyCmd := exec.Command("git", "-C", workspace, "status", "--porcelain")
	rawDirty, err := dirtyCmd.CombinedOutput()
	if err != nil {
		return buildMetadata{}, fmt.Errorf("inspect DevTool source status: %w: %s", err, strings.TrimSpace(string(rawDirty)))
	}
	return buildMetadata{Version: version, Commit: commit, Dirty: len(strings.TrimSpace(string(rawDirty))) != 0}, nil
}

func devtoolBuildArgs(output string, metadata buildMetadata, strip bool) []string {
	ldflags := []string{
		"-X", "github.com/thinkerqaq/devtool/internal/buildinfo.Version=" + metadata.Version,
		"-X", "github.com/thinkerqaq/devtool/internal/buildinfo.Commit=" + metadata.Commit,
		"-X", fmt.Sprintf("github.com/thinkerqaq/devtool/internal/buildinfo.Dirty=%t", metadata.Dirty),
	}
	if strip {
		ldflags = append([]string{"-s", "-w"}, ldflags...)
	}
	return []string{
		"build",
		"-trimpath",
		"-ldflags=" + strings.Join(ldflags, " "),
		"-o", output,
		"./cmd/devtool",
	}
}

func shortCommit(commit string) string {
	commit = strings.TrimSpace(commit)
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func packageBinaryName(version string) string {
	name := fmt.Sprintf("devtool-%s-%s-%s", version, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}
