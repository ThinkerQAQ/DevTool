package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	credentialcontract "github.com/thinkerqaq/devtool/sdk/credential"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	"github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "credential.store.file"

type Extension struct {
	mu   sync.Mutex
	path string
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindInfrastructure,
		Provides: []string{credentialcontract.StoreServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	raw, ok := settings["path"]
	if !ok {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return fmt.Errorf("credential.store.file settings.path must be a string")
	}
	e.path = strings.TrimSpace(value)
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(credentialcontract.StoreServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch method {
	case credentialcontract.MethodGet:
		var request credentialcontract.StoreGetRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode credential store get: %w", err)
		}
		values, err := e.read()
		if err != nil {
			return nil, err
		}
		value, found := values[strings.TrimSpace(request.Key)]
		return json.Marshal(credentialcontract.StoreGetResponse{Found: found, Value: value})
	case credentialcontract.MethodPut:
		var request credentialcontract.StorePutRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode credential store put: %w", err)
		}
		key := strings.TrimSpace(request.Key)
		if key == "" {
			return nil, fmt.Errorf("credential store key is required")
		}
		values, err := e.read()
		if err != nil {
			return nil, err
		}
		if request.Value == "" {
			delete(values, key)
		} else {
			values[key] = request.Value
		}
		if err := e.write(values); err != nil {
			return nil, err
		}
		return json.RawMessage(`{"stored":true}`), nil
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) storePath() (string, error) {
	if value := strings.TrimSpace(e.path); value != "" {
		return value, nil
	}
	if root := strings.TrimSpace(os.Getenv("DEVTOOL_STATE_DIR")); root != "" {
		return filepath.Join(root, "credentials.json"), nil
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve credential state directory: %w", err)
	}
	return filepath.Join(config, "devtool", "credentials.json"), nil
}

func (e *Extension) read() (map[string]string, error) {
	path, err := e.storePath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read credential store: %w", err)
	}
	values := map[string]string{}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("decode credential store: %w", err)
		}
	}
	return values, nil
}

func (e *Extension) write(values map[string]string) error {
	path, err := e.storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credential store directory: %w", err)
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write credential store: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return fmt.Errorf("secure credential store: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace credential store: %w", err)
	}
	return nil
}
