package gitworkspace

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/readiness"
	service "github.com/thinkerqaq/devtool/sdk/service"
	workspacecontract "github.com/thinkerqaq/devtool/sdk/workspace"
)

const ExtensionID = "workspace.git"

type Extension struct{}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:        ExtensionID,
		Kind:      extensioncontract.KindInfrastructure,
		Provides:  []string{workspacecontract.ServiceName},
		Readiness: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(
		workspacecontract.ServiceName,
		ExtensionID,
		service.Func(e.Invoke),
	)
}

func (e *Extension) CheckReadiness(
	ctx context.Context,
	request readiness.Request,
) (readiness.Report, error) {
	report := readiness.Report{Provider: ExtensionID}
	repository, err := resolveRepository(ctx, request.Root)
	if err != nil {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        readiness.KindVerificationFailed,
			Resource:    "git-worktree",
			Message:     err.Error(),
			Remediation: "Use a Git-backed project workspace and rerun devtool init.",
		})
		return report, nil
	}
	report.Ready = true
	report.Details = map[string]string{
		"repository_id": repositoryID(repository.CommonDir),
		"root":          repository.TopLevel,
	}
	return report, nil
}

func (e *Extension) Invoke(
	ctx context.Context,
	method string,
	payload json.RawMessage,
) (json.RawMessage, error) {
	switch method {
	case workspacecontract.MethodCreate:
		var request workspacecontract.CreateRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode workspace create request: %w", err)
		}
		response, err := e.create(ctx, request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)

	case workspacecontract.MethodList:
		var request workspacecontract.ListRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode workspace list request: %w", err)
		}
		items, err := e.list(ctx, request.Root)
		if err != nil {
			return nil, err
		}
		return json.Marshal(workspacecontract.ListResponse{Workspaces: items})

	case workspacecontract.MethodInspect:
		var request workspacecontract.InspectRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode workspace inspect request: %w", err)
		}
		item, err := e.inspect(ctx, request.Root, request.WorkspaceID)
		if err != nil {
			return nil, err
		}
		return json.Marshal(workspacecontract.InspectResponse{Workspace: item})

	case workspacecontract.MethodRemove:
		var request workspacecontract.RemoveRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode workspace remove request: %w", err)
		}
		response, err := e.remove(ctx, request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)

	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

type repositoryInfo struct {
	TopLevel  string
	CommonDir string
}

type worktreeRecord struct {
	Root     string
	Head     string
	Branch   string
	Detached bool
	Prunable bool
}

func (e *Extension) create(
	ctx context.Context,
	request workspacecontract.CreateRequest,
) (workspacecontract.CreateResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return workspacecontract.CreateResponse{}, errors.New("workspace name is required")
	}
	if _, err := gitOutput(ctx, request.Root, "check-ref-format", "--branch", name); err != nil {
		return workspacecontract.CreateResponse{}, fmt.Errorf("invalid workspace branch %q: %w", name, err)
	}

	repository, err := resolveRepository(ctx, request.Root)
	if err != nil {
		return workspacecontract.CreateResponse{}, err
	}
	records, err := listWorktrees(ctx, repository.TopLevel)
	if err != nil {
		return workspacecontract.CreateResponse{}, err
	}
	if len(records) == 0 {
		return workspacecontract.CreateResponse{}, errors.New("Git repository has no worktrees")
	}

	for _, record := range records {
		if record.Branch == name {
			return workspacecontract.CreateResponse{}, fmt.Errorf(
				"workspace branch %q is already checked out at %s",
				name,
				record.Root,
			)
		}
	}

	base := worktreeBase(records[0].Root)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return workspacecontract.CreateResponse{}, fmt.Errorf("create worktree base: %w", err)
	}
	target := filepath.Join(base, worktreeDirectoryName(name))
	if _, err := os.Stat(target); err == nil {
		return workspacecontract.CreateResponse{}, fmt.Errorf("workspace path already exists: %s", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return workspacecontract.CreateResponse{}, fmt.Errorf("inspect workspace path %s: %w", target, err)
	}

	exists, err := branchExists(ctx, repository.TopLevel, name)
	if err != nil {
		return workspacecontract.CreateResponse{}, err
	}
	if exists {
		if err := gitRun(ctx, repository.TopLevel, "worktree", "add", target, name); err != nil {
			return workspacecontract.CreateResponse{}, err
		}
	} else {
		revision := strings.TrimSpace(request.Revision)
		if revision == "" {
			revision = "HEAD"
		}
		if err := gitRun(
			ctx,
			repository.TopLevel,
			"worktree",
			"add",
			"-b",
			name,
			target,
			revision,
		); err != nil {
			return workspacecontract.CreateResponse{}, err
		}
	}

	items, err := e.list(ctx, repository.TopLevel)
	if err != nil {
		return workspacecontract.CreateResponse{}, err
	}
	target, _ = canonicalPath(target)
	for _, item := range items {
		if item.Identity.Root == target {
			return workspacecontract.CreateResponse{Workspace: item}, nil
		}
	}
	return workspacecontract.CreateResponse{}, fmt.Errorf(
		"created worktree %s is not present in Git worktree list",
		target,
	)
}

func (e *Extension) list(
	ctx context.Context,
	root string,
) ([]workspacecontract.Descriptor, error) {
	repository, err := resolveRepository(ctx, root)
	if err != nil {
		return nil, err
	}
	records, err := listWorktrees(ctx, repository.TopLevel)
	if err != nil {
		return nil, err
	}

	items := make([]workspacecontract.Descriptor, 0, len(records))
	for index, record := range records {
		if record.Prunable {
			continue
		}
		descriptor, err := describeWorktree(
			ctx,
			repository,
			record,
			index == 0,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, descriptor)
	}
	return items, nil
}

func (e *Extension) inspect(
	ctx context.Context,
	root string,
	workspaceID string,
) (workspacecontract.Descriptor, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return workspacecontract.Descriptor{}, errors.New("workspace id is required")
	}
	items, err := e.list(ctx, root)
	if err != nil {
		return workspacecontract.Descriptor{}, err
	}
	for _, item := range items {
		if item.Identity.WorkspaceID == workspaceID {
			return item, nil
		}
	}
	return workspacecontract.Descriptor{}, fmt.Errorf("workspace %q was not found", workspaceID)
}

func (e *Extension) remove(
	ctx context.Context,
	request workspacecontract.RemoveRequest,
) (workspacecontract.RemoveResponse, error) {
	item, err := e.inspect(ctx, request.Root, request.WorkspaceID)
	if err != nil {
		return workspacecontract.RemoveResponse{}, err
	}
	if item.Primary {
		return workspacecontract.RemoveResponse{}, errors.New("primary workspace cannot be removed")
	}
	if err := gitRun(ctx, request.Root, "worktree", "remove", item.Identity.Root); err != nil {
		return workspacecontract.RemoveResponse{}, err
	}
	return workspacecontract.RemoveResponse{
		WorkspaceID: item.Identity.WorkspaceID,
		Removed:     true,
	}, nil
}

func describeWorktree(
	ctx context.Context,
	repository repositoryInfo,
	record worktreeRecord,
	primary bool,
) (workspacecontract.Descriptor, error) {
	root, err := canonicalPath(record.Root)
	if err != nil {
		return workspacecontract.Descriptor{}, err
	}
	gitDir, err := gitOutput(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return workspacecontract.Descriptor{}, err
	}
	gitDir, err = canonicalPath(gitDir)
	if err != nil {
		return workspacecontract.Descriptor{}, err
	}

	name := strings.TrimSpace(record.Branch)
	if name == "" {
		name = filepath.Base(root)
	}
	revision := strings.TrimSpace(record.Branch)
	if revision == "" {
		revision = strings.TrimSpace(record.Head)
	}
	return workspacecontract.Descriptor{
		Provider: ExtensionID,
		Name:     name,
		Identity: workspacecontract.Identity{
			RepositoryID: repositoryID(repository.CommonDir),
			WorkspaceID:  workspaceID(gitDir),
			Root:         root,
		},
		Revision: revision,
		Primary:  primary,
	}, nil
}

func resolveRepository(ctx context.Context, root string) (repositoryInfo, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return repositoryInfo{}, errors.New("project root is required")
	}
	topLevel, err := gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return repositoryInfo{}, err
	}
	topLevel, err = canonicalPath(topLevel)
	if err != nil {
		return repositoryInfo{}, err
	}

	commonDir, err := gitOutput(ctx, root, "rev-parse", "--git-common-dir")
	if err != nil {
		return repositoryInfo{}, err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	commonDir, err = canonicalPath(commonDir)
	if err != nil {
		return repositoryInfo{}, err
	}
	return repositoryInfo{TopLevel: topLevel, CommonDir: commonDir}, nil
}

func listWorktrees(ctx context.Context, root string) ([]worktreeRecord, error) {
	raw, err := gitOutput(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var records []worktreeRecord
	var current worktreeRecord
	flush := func() {
		if strings.TrimSpace(current.Root) == "" {
			return
		}
		current.Branch = strings.TrimPrefix(current.Branch, "refs/heads/")
		records = append(records, current)
		current = worktreeRecord{}
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Root = value
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = value
		case "detached":
			current.Detached = true
		case "prunable":
			current.Prunable = true
		}
	}
	flush()
	return records, nil
}

func branchExists(ctx context.Context, root, branch string) (bool, error) {
	cmd := gitCommand(
		ctx,
		root,
		"show-ref",
		"--verify",
		"--quiet",
		"refs/heads/"+branch,
	)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check Git branch %q: %w", branch, err)
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := gitCommand(ctx, root, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf(
			"git %s: %w: %s",
			strings.Join(args, " "),
			err,
			strings.TrimSpace(string(out)),
		)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRun(ctx context.Context, root string, args ...string) error {
	_, err := gitOutput(ctx, root, args...)
	return err
}

func gitCommand(ctx context.Context, root string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(
		os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	return cmd
}

func canonicalPath(value string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = filepath.Clean(resolved)
	}
	return absolute, nil
}

func worktreeBase(primaryRoot string) string {
	return filepath.Join(
		filepath.Dir(primaryRoot),
		".devtool-worktrees",
		filepath.Base(primaryRoot),
	)
}

func worktreeDirectoryName(name string) string {
	var builder strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	value := strings.Trim(builder.String(), "-.")
	if value == "" {
		value = "workspace"
	}
	if len(value) > 48 {
		value = value[:48]
	}
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%s-%x", value, sum[:4])
}

func repositoryID(commonDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(commonDir)))
	return fmt.Sprintf("git-repo-%x", sum[:8])
}

func workspaceID(gitDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(gitDir)))
	return fmt.Sprintf("git-worktree-%x", sum[:8])
}
