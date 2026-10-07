package devcontrol

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/project"
)

type installLayout struct {
	BinPath     string
	DataRoot    string
	VersionsDir string
	StatePath   string
}

type releaseRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	SHA256  string `json:"sha256"`
	Binary  string `json:"binary"`
}

type installState struct {
	SchemaVersion int         `json:"schema_version"`
	Active        releaseRef  `json:"active"`
	Previous      *releaseRef `json:"previous,omitempty"`
}

func activate(ctx project.Context, workspace string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("DevTool activation on native Windows is not supported yet; use WSL")
	}
	layout, err := defaultInstallLayout()
	if err != nil {
		return err
	}
	active, previous, err := activateVerifiedCandidate(workspace, layout)
	if err != nil {
		return err
	}
	message := fmt.Sprintf("Activated DevTool %s (%s) at %s", active.Version, shortCommit(active.Commit), layout.BinPath)
	if previous != nil {
		message += fmt.Sprintf("; rollback=%s (%s)", previous.Version, shortCommit(previous.Commit))
	}
	return ctx.Emit("result", message)
}

func rollback(ctx project.Context) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("DevTool rollback on native Windows is not supported yet; use WSL")
	}
	layout, err := defaultInstallLayout()
	if err != nil {
		return err
	}
	active, err := rollbackInstalled(layout)
	if err != nil {
		return err
	}
	return ctx.Emit("result", fmt.Sprintf("Rolled back to DevTool %s (%s) at %s", active.Version, shortCommit(active.Commit), layout.BinPath))
}

func defaultInstallLayout() (installLayout, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return installLayout{}, fmt.Errorf("resolve DevTool install home: %w", err)
	}
	dataRoot := filepath.Join(home, ".local", "share", "devtool")
	return installLayout{
		BinPath:     filepath.Join(home, ".local", "bin", activeBinaryName()),
		DataRoot:    dataRoot,
		VersionsDir: filepath.Join(dataRoot, "versions"),
		StatePath:   filepath.Join(dataRoot, "install-state.json"),
	}, nil
}

func activeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "devtool.exe"
	}
	return "devtool"
}

func activateVerifiedCandidate(workspace string, layout installLayout) (releaseRef, *releaseRef, error) {
	metadata, err := loadBuildMetadata(workspace)
	if err != nil {
		return releaseRef{}, nil, err
	}
	if metadata.Dirty {
		return releaseRef{}, nil, fmt.Errorf("refuse to activate DevTool from a dirty worktree; commit the source and verify again")
	}

	manifest, err := readVerificationManifest(workspace)
	if err != nil {
		return releaseRef{}, nil, err
	}
	if err := validateManifestAgainstSource(manifest, metadata); err != nil {
		return releaseRef{}, nil, err
	}

	candidatePath := filepath.Join(workspace, ".devtool", "out", manifest.Binary)
	digest, err := sha256File(candidatePath)
	if err != nil {
		return releaseRef{}, nil, err
	}
	if digest != manifest.SHA256 {
		return releaseRef{}, nil, fmt.Errorf("verified DevTool candidate digest changed: got %s want %s; run devtool verify again", digest, manifest.SHA256)
	}
	if err := verifyBinaryBuildInfo(candidatePath, manifest.Version, manifest.Commit, false); err != nil {
		return releaseRef{}, nil, err
	}

	release := releaseRef{
		ID:      releaseID(manifest.Version, manifest.Commit),
		Version: manifest.Version,
		Commit:  manifest.Commit,
		SHA256:  manifest.SHA256,
		Binary:  activeBinaryName(),
	}
	releasePath := filepath.Join(layout.VersionsDir, release.ID, release.Binary)
	if err := installVersionedArtifact(candidatePath, releasePath, manifest); err != nil {
		return releaseRef{}, nil, err
	}

	state, stateFound, err := readInstallState(layout.StatePath)
	if err != nil {
		return releaseRef{}, nil, err
	}
	var previous *releaseRef
	if stateFound {
		current := state.Active
		previous = &current
	} else {
		previous, err = snapshotExistingActive(layout)
		if err != nil {
			return releaseRef{}, nil, err
		}
	}

	nextState := installState{SchemaVersion: 1, Active: release, Previous: previous}
	if err := activateRelease(layout, release, nextState, previous); err != nil {
		return releaseRef{}, nil, err
	}
	return release, previous, nil
}

func rollbackInstalled(layout installLayout) (releaseRef, error) {
	state, found, err := readInstallState(layout.StatePath)
	if err != nil {
		return releaseRef{}, err
	}
	if !found || state.Previous == nil {
		return releaseRef{}, fmt.Errorf("no previous DevTool installation is available for rollback")
	}
	target := *state.Previous
	targetPath := filepath.Join(layout.VersionsDir, target.ID, target.Binary)
	digest, err := sha256File(targetPath)
	if err != nil {
		return releaseRef{}, fmt.Errorf("verify rollback artifact: %w", err)
	}
	if digest != target.SHA256 {
		return releaseRef{}, fmt.Errorf("rollback artifact digest mismatch: got %s want %s", digest, target.SHA256)
	}

	oldActive := state.Active
	nextState := installState{SchemaVersion: 1, Active: target, Previous: &oldActive}
	if err := activateRelease(layout, target, nextState, &oldActive); err != nil {
		return releaseRef{}, err
	}
	return target, nil
}

func readVerificationManifest(workspace string) (verificationManifest, error) {
	path := verificationManifestPath(workspace)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return verificationManifest{}, fmt.Errorf("DevTool candidate is not verified; run devtool verify before activation")
	}
	if err != nil {
		return verificationManifest{}, fmt.Errorf("read DevTool verification manifest: %w", err)
	}
	var manifest verificationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return verificationManifest{}, fmt.Errorf("decode DevTool verification manifest: %w", err)
	}
	if manifest.SchemaVersion != 1 {
		return verificationManifest{}, fmt.Errorf("unsupported DevTool verification manifest version %d", manifest.SchemaVersion)
	}
	return manifest, nil
}

func validateManifestAgainstSource(manifest verificationManifest, metadata buildMetadata) error {
	if manifest.Dirty {
		return fmt.Errorf("refuse to activate a candidate verified from dirty source; commit the source and verify again")
	}
	if manifest.Version != metadata.Version || manifest.Commit != metadata.Commit {
		return fmt.Errorf(
			"verified DevTool candidate is stale: manifest version=%s commit=%s, source version=%s commit=%s; run devtool verify again",
			manifest.Version, manifest.Commit, metadata.Version, metadata.Commit,
		)
	}
	if strings.TrimSpace(manifest.SHA256) == "" || strings.TrimSpace(manifest.Binary) == "" {
		return fmt.Errorf("DevTool verification manifest is incomplete")
	}
	return nil
}

func verifyBinaryBuildInfo(binaryPath, version, commit string, dirty bool) error {
	cmd := exec.Command(binaryPath, "version", "--json")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("smoke DevTool candidate version: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	var got struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Dirty   bool   `json:"dirty"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return fmt.Errorf("decode DevTool candidate version smoke: %w", err)
	}
	if got.Version != version || got.Commit != commit || got.Dirty != dirty {
		return fmt.Errorf(
			"DevTool candidate version smoke mismatch: got version=%s commit=%s dirty=%t, want version=%s commit=%s dirty=%t",
			got.Version, got.Commit, got.Dirty, version, commit, dirty,
		)
	}
	return nil
}

func releaseID(version, commit string) string {
	return version + "+" + shortCommit(commit)
}

func installVersionedArtifact(candidatePath, releasePath string, manifest verificationManifest) error {
	if err := os.MkdirAll(filepath.Dir(releasePath), 0o755); err != nil {
		return fmt.Errorf("create DevTool version directory: %w", err)
	}
	if err := copyFileAtomic(candidatePath, releasePath, 0o755); err != nil {
		return fmt.Errorf("install versioned DevTool artifact: %w", err)
	}
	manifestPath := filepath.Join(filepath.Dir(releasePath), "manifest.json")
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := writeFileAtomic(manifestPath, raw, 0o600); err != nil {
		return fmt.Errorf("install DevTool version manifest: %w", err)
	}
	return nil
}

func snapshotExistingActive(layout installLayout) (*releaseRef, error) {
	info, err := os.Stat(layout.BinPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect current DevTool binary: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("current DevTool path is a directory: %s", layout.BinPath)
	}
	digest, err := sha256File(layout.BinPath)
	if err != nil {
		return nil, err
	}
	ref := releaseRef{
		ID:      "legacy-" + digest[:12],
		Version: "legacy",
		Commit:  "unknown",
		SHA256:  digest,
		Binary:  activeBinaryName(),
	}
	if raw, err := exec.Command(layout.BinPath, "version", "--json").Output(); err == nil {
		var info struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		}
		version := strings.TrimSpace(info.Version)
		commit := strings.TrimSpace(info.Commit)
		if json.Unmarshal(raw, &info) == nil {
			version = strings.TrimSpace(info.Version)
			commit = strings.TrimSpace(info.Commit)
			if validVersion(version) && commit != "" {
				ref.Version = version
				ref.Commit = commit
				ref.ID = releaseID(ref.Version, ref.Commit)
			}
		}
	}
	target := filepath.Join(layout.VersionsDir, ref.ID, ref.Binary)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("create DevTool previous-version directory: %w", err)
	}
	if err := copyFileAtomic(layout.BinPath, target, 0o755); err != nil {
		return nil, fmt.Errorf("snapshot current DevTool binary: %w", err)
	}
	return &ref, nil
}

func activateRelease(layout installLayout, release releaseRef, state installState, fallback *releaseRef) error {
	source := filepath.Join(layout.VersionsDir, release.ID, release.Binary)
	digest, err := sha256File(source)
	if err != nil {
		return err
	}
	if digest != release.SHA256 {
		return fmt.Errorf("DevTool release artifact digest mismatch: got %s want %s", digest, release.SHA256)
	}
	if err := os.MkdirAll(filepath.Dir(layout.BinPath), 0o755); err != nil {
		return fmt.Errorf("create DevTool binary directory: %w", err)
	}
	if err := os.MkdirAll(layout.DataRoot, 0o755); err != nil {
		return fmt.Errorf("create DevTool data directory: %w", err)
	}

	stateRaw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	stateRaw = append(stateRaw, '\n')
	stateTmp := layout.StatePath + ".next"
	if err := os.WriteFile(stateTmp, stateRaw, 0o600); err != nil {
		return fmt.Errorf("stage DevTool install state: %w", err)
	}

	if err := copyFileAtomic(source, layout.BinPath, 0o755); err != nil {
		_ = os.Remove(stateTmp)
		return fmt.Errorf("activate DevTool binary: %w", err)
	}
	if err := os.Rename(stateTmp, layout.StatePath); err != nil {
		_ = os.Remove(stateTmp)
		if fallback != nil {
			fallbackPath := filepath.Join(layout.VersionsDir, fallback.ID, fallback.Binary)
			_ = copyFileAtomic(fallbackPath, layout.BinPath, 0o755)
		}
		return fmt.Errorf("activate DevTool install state: %w", err)
	}
	return nil
}

func readInstallState(path string) (installState, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return installState{}, false, nil
	}
	if err != nil {
		return installState{}, false, fmt.Errorf("read DevTool install state: %w", err)
	}
	var state installState
	if err := json.Unmarshal(raw, &state); err != nil {
		return installState{}, false, fmt.Errorf("decode DevTool install state: %w", err)
	}
	if state.SchemaVersion != 1 {
		return installState{}, false, fmt.Errorf("unsupported DevTool install state version %d", state.SchemaVersion)
	}
	return state, true, nil
}

func copyFileAtomic(source, target string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".next-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func writeFileAtomic(path string, raw []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".next-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
