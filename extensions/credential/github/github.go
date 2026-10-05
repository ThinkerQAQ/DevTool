package github

import (
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
	"github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "credential.github"

type Extension struct {
	clientID   string
	httpClient *http.Client
	services   extensioncontract.Registrar
}

type pendingDevice struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresAt       int64  `json:"expires_at"`
	Interval        int64  `json:"interval"`
	LastPoll        int64  `json:"last_poll,omitempty"`
}

func New() *Extension {
	return &Extension{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindInfrastructure,
		Provides: []string{credentialcontract.ServiceName},
		Requires: []string{credentialcontract.StoreServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	raw, ok := settings["client_id"]
	if !ok {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return fmt.Errorf("credential.github settings.client_id must be a string")
	}
	e.clientID = strings.TrimSpace(value)
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideService(credentialcontract.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != credentialcontract.MethodResolve {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request credentialcontract.ResolveRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode GitHub credential request: %w", err)
	}
	response, err := e.resolve(ctx, request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

func (e *Extension) resolve(ctx context.Context, request credentialcontract.ResolveRequest) (credentialcontract.ResolveResponse, error) {
	host := strings.TrimSpace(request.Host)
	if host == "" {
		host = "github.com"
	}
	if host != "github.com" {
		return credentialcontract.ResolveResponse{}, fmt.Errorf("%s does not support host %q", ExtensionID, host)
	}

	if token := firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")); token != "" {
		return credentialcontract.ResolveResponse{Ready: true, Secret: token, Source: "env"}, nil
	}
	if token := gitCredential(ctx, request.Root); token != "" {
		return credentialcontract.ResolveResponse{Ready: true, Secret: token, Source: "git-credential"}, nil
	}
	if token, ok, err := e.storeGet(ctx, "github.com/token"); err != nil {
		return credentialcontract.ResolveResponse{}, err
	} else if ok && strings.TrimSpace(token) != "" {
		return credentialcontract.ResolveResponse{Ready: true, Secret: strings.TrimSpace(token), Source: "credential-store"}, nil
	}

	if raw, ok, err := e.storeGet(ctx, "github.com/device"); err != nil {
		return credentialcontract.ResolveResponse{}, err
	} else if ok && strings.TrimSpace(raw) != "" {
		var pending pendingDevice
		if json.Unmarshal([]byte(raw), &pending) == nil {
			return e.pollDevice(ctx, pending)
		}
	}

	clientID := firstNonEmpty(strings.TrimSpace(e.clientID), strings.TrimSpace(os.Getenv("DEVTOOL_GITHUB_CLIENT_ID")))
	if clientID == "" {
		return credentialcontract.ResolveResponse{Authorization: &credentialcontract.Authorization{
			Status: "configuration_required",
			Reason: "GitHub device authorization requires a registered DevTool OAuth App client_id",
		}}, nil
	}
	return e.beginDevice(ctx, clientID, request.Scopes)
}

func (e *Extension) beginDevice(ctx context.Context, clientID string, scopes []string) (credentialcontract.ResolveResponse, error) {
	scope := "repo"
	if len(scopes) != 0 {
		scope = strings.Join(scopes, " ")
	}
	form := url.Values{"client_id": {clientID}, "scope": {scope}}
	var response struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int64  `json:"expires_in"`
		Interval        int64  `json:"interval"`
	}
	if err := e.postForm(ctx, "https://github.com/login/device/code", form, &response); err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	now := time.Now().Unix()
	pending := pendingDevice{
		DeviceCode:      response.DeviceCode,
		UserCode:        response.UserCode,
		VerificationURI: response.VerificationURI,
		ExpiresAt:       now + response.ExpiresIn,
		Interval:        max64(response.Interval, 5),
	}
	raw, _ := json.Marshal(pending)
	if err := e.storePut(ctx, "github.com/device", string(raw)); err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	return authResponse(pending, "authorization_required"), nil
}

func (e *Extension) pollDevice(ctx context.Context, p pendingDevice) (credentialcontract.ResolveResponse, error) {
	now := time.Now().Unix()
	if p.ExpiresAt <= now {
		_ = e.storePut(ctx, "github.com/device", "")
		return credentialcontract.ResolveResponse{Authorization: &credentialcontract.Authorization{Status: "expired", Reason: "GitHub device authorization expired"}}, nil
	}
	if p.LastPoll != 0 && now-p.LastPoll < p.Interval {
		return authResponse(p, "authorization_required"), nil
	}
	clientID := firstNonEmpty(strings.TrimSpace(e.clientID), strings.TrimSpace(os.Getenv("DEVTOOL_GITHUB_CLIENT_ID")))
	if clientID == "" {
		return credentialcontract.ResolveResponse{Authorization: &credentialcontract.Authorization{Status: "configuration_required", Reason: "GitHub OAuth client_id is unavailable"}}, nil
	}
	form := url.Values{
		"client_id":   {clientID},
		"device_code": {p.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	var response struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := e.postForm(ctx, "https://github.com/login/oauth/access_token", form, &response); err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	if token := strings.TrimSpace(response.AccessToken); token != "" {
		if err := e.storePut(ctx, "github.com/token", token); err != nil {
			return credentialcontract.ResolveResponse{}, err
		}
		_ = e.storePut(ctx, "github.com/device", "")
		return credentialcontract.ResolveResponse{Ready: true, Secret: token, Source: "device-flow"}, nil
	}
	switch response.Error {
	case "", "authorization_pending":
		p.LastPoll = now
	case "slow_down":
		p.Interval += 5
		p.LastPoll = now
	case "access_denied", "expired_token":
		_ = e.storePut(ctx, "github.com/device", "")
		return credentialcontract.ResolveResponse{Authorization: &credentialcontract.Authorization{Status: response.Error, Reason: "GitHub device authorization did not complete"}}, nil
	default:
		return credentialcontract.ResolveResponse{}, fmt.Errorf("GitHub device authorization: %s", response.Error)
	}
	raw, _ := json.Marshal(p)
	if err := e.storePut(ctx, "github.com/device", string(raw)); err != nil {
		return credentialcontract.ResolveResponse{}, err
	}
	return authResponse(p, "authorization_required"), nil
}

func authResponse(p pendingDevice, status string) credentialcontract.ResolveResponse {
	return credentialcontract.ResolveResponse{Authorization: &credentialcontract.Authorization{
		Status: status, VerificationURI: p.VerificationURI, UserCode: p.UserCode, ExpiresAt: p.ExpiresAt,
	}}
}

func (e *Extension) postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub authorization HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.Unmarshal(raw, out)
}

func (e *Extension) storeGet(ctx context.Context, key string) (string, bool, error) {
	invoker, ok := e.services.Service(credentialcontract.StoreServiceName)
	if !ok {
		return "", false, fmt.Errorf("service %q is not configured", credentialcontract.StoreServiceName)
	}
	payload, _ := json.Marshal(credentialcontract.StoreGetRequest{Key: key})
	raw, err := invoker.Invoke(ctx, credentialcontract.MethodGet, payload)
	if err != nil {
		return "", false, err
	}
	var response credentialcontract.StoreGetResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", false, err
	}
	return response.Value, response.Found, nil
}

func (e *Extension) storePut(ctx context.Context, key, value string) error {
	invoker, ok := e.services.Service(credentialcontract.StoreServiceName)
	if !ok {
		return fmt.Errorf("service %q is not configured", credentialcontract.StoreServiceName)
	}
	payload, _ := json.Marshal(credentialcontract.StorePutRequest{Key: key, Value: value})
	_, err := invoker.Invoke(ctx, credentialcontract.MethodPut, payload)
	return err
}

func gitCredential(ctx context.Context, root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "git", "credential", "fill")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	cmd.Stdin = strings.NewReader("protocol=https\nhost=github.com\n\n")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key == "password" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
