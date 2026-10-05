package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	service "github.com/thinkerqaq/devtool/sdk/service"
	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/scm"
)

const ExtensionID = "scm.github"

type Extension struct {
	httpClient *http.Client
}

func New() *Extension {
	return &Extension{httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindInfrastructure,
		Provides:   []string{scm.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	if err := reg.ProvideService(scm.ServiceName, ExtensionID, service.Func(e.Invoke)); err != nil {
		return err
	}
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case scm.MethodDoctor:
		var request scm.Request
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm doctor request: %w", err)
		}
		response := e.doctor(ctx, request.Root)
		return json.Marshal(response)
	case scm.MethodStatus:
		var request scm.Request
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm status request: %w", err)
		}
		response, err := e.status(ctx, request.Root)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	case scm.MethodCommit:
		var request scm.CommitRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm commit request: %w", err)
		}
		response, err := e.commit(ctx, request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	case scm.MethodPush:
		var request scm.PushRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm push request: %w", err)
		}
		response, err := e.push(ctx, request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	case scm.MethodPublish:
		var request scm.PublishRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm publish request: %w", err)
		}
		response, err := e.publish(ctx, request)
		if err != nil {
			return nil, err
		}
		return json.Marshal(response)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) InvokeService(ctx context.Context, serviceName, method string, payload json.RawMessage) (json.RawMessage, error) {
	if serviceName != scm.ServiceName {
		return nil, fmt.Errorf("%s does not provide service %q", ExtensionID, serviceName)
	}
	return e.Invoke(ctx, method, payload)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{
		tool("scm_doctor", "Check source-control remote and credentials before publishing.", map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}),
		tool("scm_status", "Inspect the current branch, worktree changes, and upstream divergence.", map[string]any{
			"type": "object", "properties": map[string]any{},
		}),
		tool("scm_commit", "Stage current changes and create one source-control commit.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required": []string{"message"},
		}),
		tool("scm_push", "Push the current branch without creating a pull request.", map[string]any{
			"type": "object", "properties": map[string]any{},
		}),
		tool("scm_publish", "Push the current branch, create or reuse a GitHub pull request, and optionally merge it.", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"base":  map[string]any{"type": "string", "default": "main"},
				"title": map[string]any{"type": "string"},
				"body":  map[string]any{"type": "string"},
				"merge": map[string]any{"type": "boolean", "default": false},
			},
		}),
	}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	switch name {
	case "scm_doctor":
		response := e.doctor(ctx, session.ProjectRoot)
		return toolResult(response)
	case "scm_status":
		response, err := e.status(ctx, session.ProjectRoot)
		if err != nil {
			return nil, err
		}
		return toolResult(response)
	case "scm_commit":
		var request scm.CommitRequest
		if err := json.Unmarshal(args, &request); err != nil {
			return nil, fmt.Errorf("decode scm_commit arguments: %w", err)
		}
		request.Root = session.ProjectRoot
		response, err := e.commit(ctx, request)
		if err != nil {
			return nil, err
		}
		return toolResult(response)
	case "scm_push":
		response, err := e.push(ctx, scm.PushRequest{Root: session.ProjectRoot})
		if err != nil {
			return nil, err
		}
		return toolResult(response)
	case "scm_publish":
		var request scm.PublishRequest
		if len(args) != 0 {
			if err := json.Unmarshal(args, &request); err != nil {
				return nil, fmt.Errorf("decode scm_publish arguments: %w", err)
			}
		}
		request.Root = session.ProjectRoot
		response, err := e.publish(ctx, request)
		if err != nil {
			return nil, err
		}
		return toolResult(response)
	default:
		return nil, fmt.Errorf("unknown SCM tool %q", name)
	}
}

func (e *Extension) doctor(ctx context.Context, root string) scm.DoctorResponse {
	remote, err := gitOutput(ctx, root, "config", "--get", "remote.origin.url")
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Reason: err.Error()}
	}
	owner, repo, err := parseGitHubRemote(remote)
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: remote, Reason: err.Error()}
	}
	_, token, err := githubCredential(ctx, root)
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: owner + "/" + repo, Reason: err.Error()}
	}
	if strings.TrimSpace(token) == "" {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: owner + "/" + repo, Reason: "GitHub credential is unavailable"}
	}
	var identity struct {
		Login string `json:"login"`
	}
	if err := e.githubJSON(ctx, token, http.MethodGet, "/user", nil, &identity); err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: owner + "/" + repo, Reason: "GitHub credential validation: " + err.Error()}
	}
	return scm.DoctorResponse{Provider: ExtensionID, Ready: true, Remote: owner + "/" + repo}
}

func (e *Extension) publish(ctx context.Context, request scm.PublishRequest) (scm.PublishResponse, error) {
	start := time.Now()
	root := strings.TrimSpace(request.Root)
	if root == "" {
		return scm.PublishResponse{}, fmt.Errorf("project root is required")
	}
	remote, err := gitOutput(ctx, root, "config", "--get", "remote.origin.url")
	if err != nil {
		return scm.PublishResponse{}, err
	}
	owner, repo, err := parseGitHubRemote(remote)
	if err != nil {
		return scm.PublishResponse{}, err
	}
	_, token, err := githubCredential(ctx, root)
	if err != nil {
		return scm.PublishResponse{}, fmt.Errorf("GitHub credential preflight: %w", err)
	}
	var identity struct {
		Login string `json:"login"`
	}
	if err := e.githubJSON(ctx, token, http.MethodGet, "/user", nil, &identity); err != nil {
		return scm.PublishResponse{}, fmt.Errorf("GitHub credential validation: %w", err)
	}

	branch, err := gitOutput(ctx, root, "branch", "--show-current")
	if err != nil {
		return scm.PublishResponse{}, err
	}
	if strings.TrimSpace(branch) == "" {
		return scm.PublishResponse{}, fmt.Errorf("cannot publish detached HEAD")
	}
	base := strings.TrimSpace(request.Base)
	if base == "" {
		base = "main"
	}

	status, err := e.status(ctx, root)
	if err != nil {
		return scm.PublishResponse{}, err
	}
	if !status.Clean {
		return scm.PublishResponse{}, fmt.Errorf("worktree has uncommitted changes; commit them before publish")
	}
	pushResponse, err := e.push(ctx, scm.PushRequest{Root: root})
	if err != nil {
		return scm.PublishResponse{}, err
	}
	commit := pushResponse.Commit

	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "Update " + branch
	}

	pr, err := e.findOrCreatePR(ctx, token, owner, repo, branch, base, title, request.Body)
	if err != nil {
		return scm.PublishResponse{}, err
	}
	response := scm.PublishResponse{
		Provider:  ExtensionID,
		Branch:    branch,
		Commit:    commit,
		PRNumber:  pr.Number,
		PRURL:     pr.HTMLURL,
		DurationM: time.Since(start).Milliseconds(),
	}
	if request.Merge {
		merged, err := e.mergePR(ctx, token, owner, repo, pr.Number, commit)
		if err != nil {
			return scm.PublishResponse{}, err
		}
		response.Merged = merged
		response.DurationM = time.Since(start).Milliseconds()
	}
	return response, nil
}

type pullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

func (e *Extension) findOrCreatePR(ctx context.Context, token, owner, repo, branch, base, title, body string) (pullRequest, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+branch)
	query.Set("base", base)
	var existing []pullRequest
	if err := e.githubJSON(ctx, token, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls?%s", owner, repo, query.Encode()), nil, &existing); err != nil {
		return pullRequest{}, err
	}
	if len(existing) != 0 {
		return existing[0], nil
	}
	payload := map[string]any{"title": title, "head": branch, "base": base, "body": body}
	var created pullRequest
	if err := e.githubJSON(ctx, token, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), payload, &created); err != nil {
		return pullRequest{}, err
	}
	return created, nil
}

func (e *Extension) mergePR(ctx context.Context, token, owner, repo string, number int, commit string) (bool, error) {
	payload := map[string]any{"sha": commit, "merge_method": "merge"}
	var response struct {
		Merged  bool   `json:"merged"`
		Message string `json:"message"`
	}
	if err := e.githubJSON(ctx, token, http.MethodPut, fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, number), payload, &response); err != nil {
		return false, err
	}
	if !response.Merged {
		return false, fmt.Errorf("GitHub did not merge PR #%d: %s", number, response.Message)
	}
	return true, nil
}

func (e *Extension) githubJSON(ctx context.Context, token, method, path string, input any, output any) error {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s %s: %s", method, path, strings.TrimSpace(string(raw)))
	}
	if output != nil && len(raw) != 0 {
		if err := json.Unmarshal(raw, output); err != nil {
			return err
		}
	}
	return nil
}

func gitOutput(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRun(ctx context.Context, root string, args ...string) error {
	_, err := gitOutput(ctx, root, args...)
	return err
}

func githubCredential(ctx context.Context, root string) (string, string, error) {
	credentialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(credentialCtx, "git", "credential", "fill")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
	out, err := cmd.Output()
	if err != nil {
		if credentialCtx.Err() != nil {
			return "", "", fmt.Errorf("GitHub credential lookup timed out")
		}
		return "", "", fmt.Errorf("GitHub credential unavailable: %w", err)
	}
	var username, password string
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "username":
			username = value
		case "password":
			password = value
		}
	}
	if strings.TrimSpace(password) == "" {
		return "", "", fmt.Errorf("GitHub credential unavailable")
	}
	return username, password, nil
}

func parseGitHubRemote(remote string) (string, string, error) {
	value := strings.TrimSpace(remote)
	value = strings.TrimSuffix(value, ".git")
	switch {
	case strings.HasPrefix(value, "https://github.com/"):
		value = strings.TrimPrefix(value, "https://github.com/")
	case strings.HasPrefix(value, "http://github.com/"):
		value = strings.TrimPrefix(value, "http://github.com/")
	case strings.HasPrefix(value, "git@github.com:"):
		value = strings.TrimPrefix(value, "git@github.com:")
	default:
		return "", "", fmt.Errorf("unsupported GitHub remote %q", remote)
	}
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid GitHub remote %q", remote)
	}
	return parts[0], parts[1], nil
}

func tool(name, description string, inputSchema map[string]any) agentsdk.Tool {
	raw, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": inputSchema,
	})
	return agentsdk.Tool{Name: name, Definition: raw}
}

func toolResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	result, err := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": string(raw)}},
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
