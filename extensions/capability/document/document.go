package document

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	agentsdk "github.com/thinkerqaq/devtool/sdk/agent"
	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "capability.document"

type Extension struct {
	services extensioncontract.Registrar
}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:         ExtensionID,
		Kind:       extensioncontract.KindCapability,
		Requires:   []string{documentcontract.ServiceName},
		AgentTools: true,
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	e.services = reg
	return reg.ProvideAgentTools(ExtensionID, e)
}

func (e *Extension) ListTools(context.Context, agentsdk.Session) ([]agentsdk.Tool, error) {
	return []agentsdk.Tool{
		tool(
			"document_context",
			"Build structured document context or traverse a whole document with explicit stateless review coverage.",
			map[string]any{
				"objective": map[string]any{
					"type":        "string",
					"description": "What the agent needs to understand or review.",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Project-relative document path.",
				},
				"section": map[string]any{
					"type":        "string",
					"description": "Optional exact section title or numbered section key such as 1.3.",
				},
				"include_content": map[string]any{
					"type":        "boolean",
					"description": "Include the exact selected section source in the response.",
				},
				"review": map[string]any{
					"type":        "boolean",
					"description": "Build or continue a deterministic whole-document review traversal over top-level sections.",
				},
				"cursor": map[string]any{
					"type":        "string",
					"description": "Opaque continuation cursor returned by a previous review-mode call.",
				},
			},
			[]string{"objective", "path"},
		),
	}, nil
}

type documentContextInput struct {
	Objective      string `json:"objective"`
	Path           string `json:"path"`
	Section        string `json:"section,omitempty"`
	IncludeContent bool   `json:"include_content,omitempty"`
	Review         bool   `json:"review,omitempty"`
	Cursor         string `json:"cursor,omitempty"`
}

type reviewCursor struct {
	Path      string `json:"path"`
	Next      int    `json:"next"`
	Signature string `json:"signature"`
}

type reviewSection struct {
	Key       string `json:"key,omitempty"`
	Title     string `json:"title"`
	Level     int    `json:"level"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type reviewState struct {
	TotalSections     int             `json:"total_sections"`
	CurrentSection    *reviewSection  `json:"current_section,omitempty"`
	CoveredSections   []reviewSection `json:"covered_sections"`
	RemainingSections []reviewSection `json:"remaining_sections"`
	NextSection       *reviewSection  `json:"next_section,omitempty"`
	NextCursor        string          `json:"next_cursor,omitempty"`
	Complete          bool            `json:"complete"`
}

func (e *Extension) CallTool(ctx context.Context, session agentsdk.Session, name string, args json.RawMessage) (json.RawMessage, error) {
	if strings.TrimSpace(name) != "document_context" {
		return nil, fmt.Errorf("unknown document capability tool %q", name)
	}
	if e.services == nil {
		return nil, fmt.Errorf("document capability service registry is unavailable")
	}

	var input documentContextInput
	if err := decodeArgs(args, &input); err != nil {
		return nil, err
	}
	input.Objective = strings.TrimSpace(input.Objective)
	if input.Objective == "" {
		return nil, fmt.Errorf("document_context objective is required")
	}
	input.Path = strings.TrimSpace(input.Path)
	if input.Path == "" {
		return nil, fmt.Errorf("document_context path is required")
	}
	input.Section = strings.TrimSpace(input.Section)
	input.Cursor = strings.TrimSpace(input.Cursor)

	if input.Cursor != "" && !input.Review {
		return nil, fmt.Errorf("document_context cursor requires review=true")
	}
	if input.Review && input.Section != "" {
		return nil, fmt.Errorf("document_context review mode cannot be combined with section")
	}
	if input.Review && input.IncludeContent {
		return nil, fmt.Errorf("document_context review mode controls section content; omit include_content")
	}

	invoker, ok := e.services.Service(documentcontract.ServiceName)
	if !ok {
		return nil, fmt.Errorf("service %q is not configured", documentcontract.ServiceName)
	}
	if input.Review {
		return e.callReview(ctx, session, invoker, input)
	}

	payload, err := json.Marshal(documentcontract.InspectRequest{
		Root:           session.ProjectRoot,
		Path:           input.Path,
		Section:        input.Section,
		IncludeContent: input.IncludeContent,
	})
	if err != nil {
		return nil, err
	}
	raw, err := invoker.Invoke(ctx, documentcontract.MethodInspect, payload)
	if err != nil {
		return nil, fmt.Errorf("build document context: %w", err)
	}

	return toolResult(map[string]any{
		"objective": input.Objective,
		"document":  decodeResult(raw),
	})
}

func (e *Extension) callReview(
	ctx context.Context,
	session agentsdk.Session,
	invoker service.Invoker,
	input documentContextInput,
) (json.RawMessage, error) {
	normalizedPath := normalizeDocumentPath(input.Path)

	var continuation *reviewCursor
	if input.Cursor != "" {
		decoded, err := decodeReviewCursor(input.Cursor)
		if err != nil {
			return nil, fmt.Errorf("document_context review cursor: %w", err)
		}
		if decoded.Path != normalizedPath {
			return nil, fmt.Errorf("document_context review cursor belongs to %q, not %q", decoded.Path, normalizedPath)
		}
		continuation = &decoded
	}

	document, err := inspectDocument(ctx, invoker, documentcontract.InspectRequest{
		Root: session.ProjectRoot,
		Path: input.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("build document review plan: %w", err)
	}

	sections := make([]reviewSection, 0, len(document.Outline))
	for _, section := range document.Outline {
		sections = append(sections, reviewSection{
			Key:       section.Key,
			Title:     section.Title,
			Level:     section.Level,
			StartLine: section.StartLine,
			EndLine:   section.EndLine,
		})
	}
	signature, err := reviewSignature(normalizedPath, sections)
	if err != nil {
		return nil, err
	}

	if continuation == nil {
		state, err := buildReviewState(normalizedPath, signature, sections, -1)
		if err != nil {
			return nil, err
		}
		return toolResult(map[string]any{
			"objective": input.Objective,
			"document":  document,
			"review":    state,
		})
	}
	if continuation.Signature != signature {
		return nil, fmt.Errorf("document_context review cursor is stale because the document structure changed")
	}
	if continuation.Next < 0 || continuation.Next >= len(sections) {
		return nil, fmt.Errorf("document_context review cursor section index %d is out of range", continuation.Next)
	}

	current := sections[continuation.Next]
	selected, err := inspectDocument(ctx, invoker, documentcontract.InspectRequest{
		Root:             session.ProjectRoot,
		Path:             input.Path,
		SectionStartLine: current.StartLine,
		IncludeContent:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("read document review section: %w", err)
	}
	if selected.SelectedSection == nil {
		return nil, fmt.Errorf("document review section at line %d was not returned by the provider", current.StartLine)
	}

	state, err := buildReviewState(normalizedPath, signature, sections, continuation.Next)
	if err != nil {
		return nil, err
	}
	return toolResult(map[string]any{
		"objective": input.Objective,
		"review":    state,
		"section":   selected.SelectedSection,
	})
}

func inspectDocument(
	ctx context.Context,
	invoker service.Invoker,
	request documentcontract.InspectRequest,
) (documentcontract.InspectResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return documentcontract.InspectResponse{}, err
	}
	raw, err := invoker.Invoke(ctx, documentcontract.MethodInspect, payload)
	if err != nil {
		return documentcontract.InspectResponse{}, err
	}
	var response documentcontract.InspectResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return documentcontract.InspectResponse{}, fmt.Errorf("decode document structure response: %w", err)
	}
	return response, nil
}

func buildReviewState(
	path string,
	signature string,
	sections []reviewSection,
	currentIndex int,
) (reviewState, error) {
	state := reviewState{
		TotalSections: len(sections),
	}
	if currentIndex >= 0 {
		current := sections[currentIndex]
		state.CurrentSection = &current
		state.CoveredSections = append([]reviewSection(nil), sections[:currentIndex+1]...)
	} else {
		state.CoveredSections = []reviewSection{}
	}

	nextIndex := currentIndex + 1
	if nextIndex < len(sections) {
		state.RemainingSections = append([]reviewSection(nil), sections[nextIndex:]...)
		next := sections[nextIndex]
		state.NextSection = &next
		cursor, err := encodeReviewCursor(reviewCursor{
			Path:      path,
			Next:      nextIndex,
			Signature: signature,
		})
		if err != nil {
			return reviewState{}, err
		}
		state.NextCursor = cursor
		return state, nil
	}

	state.RemainingSections = []reviewSection{}
	state.Complete = true
	return state, nil
}

func reviewSignature(path string, sections []reviewSection) (string, error) {
	raw, err := json.Marshal(struct {
		Path     string          `json:"path"`
		Sections []reviewSection `json:"sections"`
	}{
		Path:     path,
		Sections: sections,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%x", sum[:]), nil
}

func encodeReviewCursor(cursor reviewCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeReviewCursor(value string) (reviewCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return reviewCursor{}, fmt.Errorf("decode cursor: %w", err)
	}
	var cursor reviewCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return reviewCursor{}, fmt.Errorf("decode cursor payload: %w", err)
	}
	cursor.Path = normalizeDocumentPath(cursor.Path)
	if cursor.Path == "." || cursor.Signature == "" || cursor.Next < 0 {
		return reviewCursor{}, fmt.Errorf("cursor payload is incomplete")
	}
	return cursor, nil
}

func normalizeDocumentPath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func tool(name, description string, properties map[string]any, required []string) agentsdk.Tool {
	raw, _ := json.Marshal(map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"properties":           properties,
			"required":             required,
			"additionalProperties": false,
		},
	})
	return agentsdk.Tool{Name: name, Definition: raw}
}

func decodeArgs(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode document capability arguments: %w", err)
	}
	return nil
}

func decodeResult(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	return value
}

func toolResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(raw),
		}},
	})
}
