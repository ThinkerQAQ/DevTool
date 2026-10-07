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

	credentialcontract "github.com/thinkerqaq/devtool/sdk/credential"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/readiness"
	"github.com/thinkerqaq/devtool/sdk/scm"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "scm.github"

type Extension struct {
	httpClient *http.Client
	services   extensioncontract.Registrar
}

func New() *Extension {
	return &Extension{httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:        ExtensionID,
		Kind:      extensioncontract.KindInfrastructure,
		Provides:  []string{scm.ServiceName},
		Requires:  []string{credentialcontract.ServiceName},
		Readiness: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(scm.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) CheckReadiness(ctx context.Context, request readiness.Request) (readiness.Report, error) {
	report := readiness.Report{Provider: ExtensionID}
	root := strings.TrimSpace(request.Root)
	if root == "" {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:     readiness.KindConfigurationRequired,
			Resource: "project-root",
			Message:  "project root is required for SCM readiness",
		})
		return report, nil
	}

	if _, err := gitOutput(ctx, root, "config", "--get", "user.name"); err != nil {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        readiness.KindConfigurationRequired,
			Resource:    "git.user.name",
			Message:     "Git commit identity user.name is not configured",
			Remediation: "Configure git user.name for this environment, then rerun devtool init.",
		})
	}
	if _, err := gitOutput(ctx, root, "config", "--get", "user.email"); err != nil {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        readiness.KindConfigurationRequired,
			Resource:    "git.user.email",
			Message:     "Git commit identity user.email is not configured",
			Remediation: "Configure git user.email for this environment, then rerun devtool init.",
		})
	}

	doctor := e.doctor(ctx, root)
	if doctor.Remote != "" {
		if report.Details == nil {
			report.Details = map[string]string{}
		}
		report.Details["remote"] = doctor.Remote
	}
	if doctor.Authorization != nil {
		authorization := doctor.Authorization
		kind := readiness.KindAuthorizationRequired
		if authorization.Status == "configuration_required" {
			kind = readiness.KindConfigurationRequired
		}
		details := map[string]string{"status": authorization.Status}
		if authorization.VerificationURI != "" {
			details["verification_uri"] = authorization.VerificationURI
		}
		if authorization.UserCode != "" {
			details["user_code"] = authorization.UserCode
		}
		if authorization.ExpiresAt != 0 {
			details["expires_at"] = fmt.Sprint(authorization.ExpiresAt)
		}
		message := authorization.Reason
		if strings.TrimSpace(message) == "" {
			message = "GitHub authorization is required"
		}
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        kind,
			Resource:    "github.com",
			Message:     message,
			Remediation: "Complete or repair GitHub authorization, then rerun devtool init.",
			Details:     details,
		})
	} else if doctor.Reason != "" {
		report.Issues = append(report.Issues, readiness.Issue{
			Kind:        readiness.KindProviderUnavailable,
			Resource:    "github.com",
			Message:     doctor.Reason,
			Remediation: "Repair Git/GitHub connectivity or credentials, then rerun devtool init.",
		})
	}

	report.Ready = doctor.Ready && len(report.Issues) == 0
	return report, nil
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
	case scm.MethodCheckpoint:
		var request scm.CheckpointRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode scm checkpoint request: %w", err)
		}
		response, err := e.checkpoint(ctx, request)
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

func (e *Extension) doctor(ctx context.Context, root string) scm.DoctorResponse {
	remote, err := gitOutput(ctx, root, "config", "--get", "remote.origin.url")
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Reason: err.Error()}
	}
	owner, repo, err := parseGitHubRemote(remote)
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: remote, Reason: err.Error()}
	}
	credential, err := e.resolveCredential(ctx, root)
	if err != nil {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: owner + "/" + repo, Reason: err.Error()}
	}
	if !credential.Ready {
		return scm.DoctorResponse{Provider: ExtensionID, Remote: owner + "/" + repo, Authorization: credential.Authorization}
	}
	token := credential.Secret
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
	credential, err := e.resolveCredential(ctx, root)
	if err != nil {
		return scm.PublishResponse{}, fmt.Errorf("GitHub credential preflight: %w", err)
	}
	if !credential.Ready {
		return scm.PublishResponse{Provider: ExtensionID, Authorization: credential.Authorization}, nil
	}
	token := credential.Secret
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
		base, err = e.defaultBranch(ctx, token, owner, repo)
		if err != nil {
			return scm.PublishResponse{}, err
		}
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

func (e *Extension) defaultBranch(ctx context.Context, token, owner, repo string) (string, error) {
	var metadata struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := e.githubJSON(ctx, token, http.MethodGet, fmt.Sprintf("/repos/%s/%s", owner, repo), nil, &metadata); err != nil {
		return "", fmt.Errorf("resolve GitHub default branch: %w", err)
	}
	branch := strings.TrimSpace(metadata.DefaultBranch)
	if branch == "" {
		return "", fmt.Errorf("GitHub repository %s/%s has no default branch", owner, repo)
	}
	return branch, nil
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

func (e *Extension) resolveCredential(ctx context.Context, root string) (credentialcontract.ResolveResponse, error) {
	invoker, ok := e.services.Service(credentialcontract.ServiceName)
	if !ok {
		return credentialcontract.ResolveResponse{}, fmt.Errorf("service %q is not configured", credentialcontract.ServiceName)
	}
	payload, err := json.Marshal(credentialcontract.ResolveRequest{Root: root, Host: "github.com", Scopes: []string{"repo"}})
	if err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	raw, err := invoker.Invoke(ctx, credentialcontract.MethodResolve, payload)
	if err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	var response credentialcontract.ResolveResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	return response, nil
}
