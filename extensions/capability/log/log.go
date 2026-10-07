package log

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	logintelligence "github.com/thinkerqaq/devtool/sdk/logintelligence"
	service "github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "capability.log"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{logintelligence.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	definition, _ := json.Marshal(map[string]any{
		"name":        "log_context",
		"description": "Build bounded structured context from a local file or journald source for a debugging or investigation objective.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "What the agent needs to understand from the log.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Legacy shorthand for a project-relative file source.",
				},
				"source": map[string]any{
					"type":        "object",
					"description": "Structured log source. Use file for project files or journald for local service logs.",
					"properties": map[string]any{
						"kind": map[string]any{
							"type": "string",
							"enum": []string{logintelligence.SourceFile, logintelligence.SourceJournald},
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Project-relative path for kind=file.",
						},
						"unit": map[string]any{
							"type":        "string",
							"description": "systemd unit for kind=journald.",
						},
						"scope": map[string]any{
							"type": "string",
							"enum": []string{"user", "system"},
						},
						"since": map[string]any{
							"type":        "string",
							"description": "Optional journalctl-compatible start time.",
						},
						"until": map[string]any{
							"type":        "string",
							"description": "Optional journalctl-compatible end time.",
						},
						"max_entries": map[string]any{
							"type":    "integer",
							"minimum": 1,
							"maximum": 100000,
						},
					},
					"required":             []string{"kind"},
					"additionalProperties": false,
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Optional case-insensitive literal hint used to prioritize matching evidence.",
				},
				"limit": map[string]any{
					"type":    "integer",
					"minimum": 1,
					"maximum": 100,
				},
			},
			"required": []string{"objective"},
			"oneOf": []map[string]any{
				{"required": []string{"path"}},
				{"required": []string{"source"}},
			},
			"additionalProperties": false,
		},
	})
	return []agentsdk.Tool{{Name: "log_context", Definition: definition}}, nil
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (result json.RawMessage, err error) {
	if strings.TrimSpace(name) != "log_context" {
		return nil, fmt.Errorf("unknown log capability tool %q", name)
	}
	if e.services == nil {
		return nil, fmt.Errorf("log capability service registry is unavailable")
	}

	var input struct {
		Objective string                  `json:"objective"`
		Path      string                  `json:"path,omitempty"`
		Source    *logintelligence.Source `json:"source,omitempty"`
		Query     string                  `json:"query,omitempty"`
		Limit     int                     `json:"limit,omitempty"`
	}
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("decode log capability arguments: %w", err)
	}
	input.Objective = strings.TrimSpace(input.Objective)
	input.Path = strings.TrimSpace(input.Path)
	input.Query = strings.TrimSpace(input.Query)
	if input.Objective == "" {
		return nil, fmt.Errorf("log_context objective is required")
	}
	if input.Path == "" && input.Source == nil {
		return nil, fmt.Errorf("log_context path or source is required")
	}
	if input.Path != "" && input.Source != nil {
		return nil, fmt.Errorf("log_context path and source are mutually exclusive")
	}
	if input.Limit < 0 || input.Limit > 100 {
		return nil, fmt.Errorf("log_context limit must be between 1 and 100")
	}

	invoker, ok := e.services.Service(logintelligence.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", logintelligence.ServiceName)
	}
	payload, err := json.Marshal(logintelligence.AnalyzeRequest{
		Root:   session.ProjectRoot,
		Path:   input.Path,
		Source: input.Source,
		Query:  input.Query,
		Limit:  input.Limit,
	})
	if err != nil {
		return nil, err
	}

	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "log_context",
		Layer:        "capability",
		Tool:         "log_context",
		Service:      logintelligence.ServiceName,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	raw, err := invoke(ctx, invoker, payload)
	if err != nil {
		return nil, fmt.Errorf("build log context: %w", err)
	}
	var analyzed any
	if err := json.Unmarshal(raw, &analyzed); err != nil {
		analyzed = string(raw)
	}
	body, err := json.Marshal(map[string]any{
		"objective": input.Objective,
		"log":       analyzed,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(body),
		}},
	})
}

func invoke(ctx context.Context, invoker service.Invoker, payload json.RawMessage) (result json.RawMessage, err error) {
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         logintelligence.ServiceName + "." + logintelligence.MethodAnalyze,
		Layer:        "service",
		Service:      logintelligence.ServiceName,
		Method:       logintelligence.MethodAnalyze,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()
	return invoker.Invoke(ctx, logintelligence.MethodAnalyze, payload)
}
