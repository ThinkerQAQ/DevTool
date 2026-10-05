package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/thinkerqaq/devtool/sdk/scm"
)

func (e *Extension) status(ctx context.Context, root string) (scm.StatusResponse, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return scm.StatusResponse{}, fmt.Errorf("project root is required")
	}
	branch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return scm.StatusResponse{}, err
	}
	if branch == "" {
		return scm.StatusResponse{}, fmt.Errorf("cannot inspect detached HEAD")
	}
	head, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return scm.StatusResponse{}, err
	}
	changes, err := gitOutput(ctx, root, "status", "--porcelain")
	if err != nil {
		return scm.StatusResponse{}, err
	}

	response := scm.StatusResponse{
		Provider: ExtensionID,
		Branch:   branch,
		Head:     head,
		Clean:    strings.TrimSpace(changes) == "",
		Changes:  changes,
	}
	upstream, err := gitOutput(ctx, root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && strings.TrimSpace(upstream) != "" {
		counts, countErr := gitOutput(ctx, root, "rev-list", "--left-right", "--count", upstream+"...HEAD")
		if countErr == nil {
			fields := strings.Fields(counts)
			if len(fields) == 2 {
				response.Behind, _ = strconv.Atoi(fields[0])
				response.Ahead, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return response, nil
}

func (e *Extension) commit(ctx context.Context, request scm.CommitRequest) (scm.CommitResponse, error) {
	root := strings.TrimSpace(request.Root)
	if root == "" {
		return scm.CommitResponse{}, fmt.Errorf("project root is required")
	}
	message := strings.TrimSpace(request.Message)
	if message == "" {
		return scm.CommitResponse{}, fmt.Errorf("commit message is required")
	}
	status, err := e.status(ctx, root)
	if err != nil {
		return scm.CommitResponse{}, err
	}
	if status.Clean {
		return scm.CommitResponse{}, fmt.Errorf("worktree is clean")
	}
	if err := gitRun(ctx, root, "add", "-A"); err != nil {
		return scm.CommitResponse{}, err
	}
	if err := gitRun(ctx, root, "commit", "-m", message); err != nil {
		return scm.CommitResponse{}, err
	}
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return scm.CommitResponse{}, err
	}
	return scm.CommitResponse{Provider: ExtensionID, Branch: status.Branch, Commit: commit}, nil
}

func (e *Extension) push(ctx context.Context, request scm.PushRequest) (scm.PushResponse, error) {
	root := strings.TrimSpace(request.Root)
	if root == "" {
		return scm.PushResponse{}, fmt.Errorf("project root is required")
	}
	status, err := e.status(ctx, root)
	if err != nil {
		return scm.PushResponse{}, err
	}
	credential, err := e.resolveCredential(ctx, root)
	if err != nil {
		return scm.PushResponse{}, err
	}
	if !credential.Ready {
		return scm.PushResponse{Provider: ExtensionID, Branch: status.Branch, Authorization: credential.Authorization}, nil
	}
	pushCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := gitRunAuthenticated(pushCtx, root, credential.Secret, "push", "-u", "origin", "HEAD"); err != nil {
		return scm.PushResponse{}, err
	}
	commit, err := gitOutput(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return scm.PushResponse{}, err
	}
	return scm.PushResponse{Provider: ExtensionID, Branch: status.Branch, Commit: commit}, nil
}

func gitRunAuthenticated(ctx context.Context, root, token string, args ...string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.https://github.com/.extraheader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+encoded,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
