package markdown

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"

	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "document.markdown.goldmark"

var numberedHeading = regexp.MustCompile("^\\s*([0-9]+(?:\\.[0-9]+)*)(?:\\.|\\s|$|[:：])")

type Extension struct {
	roots []string
}

func New() *Extension {
	return &Extension{roots: []string{"."}}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindDocumentIntelligence,
		Provides: []string{documentcontract.ServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	raw, ok := settings["roots"]
	if !ok {
		e.roots = []string{"."}
		return nil
	}
	roots, err := stringSlice(raw)
	if err != nil {
		return fmt.Errorf("document markdown roots: %w", err)
	}
	if len(roots) == 0 {
		return fmt.Errorf("document markdown roots cannot be empty")
	}
	e.roots = roots
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(documentcontract.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != documentcontract.MethodInspect {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request documentcontract.InspectRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode document inspect request: %w", err)
	}
	response, err := e.inspect(request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

type flatSection struct {
	Key       string
	Title     string
	Level     int
	StartLine int
	EndLine   int
}

func (e *Extension) inspect(request documentcontract.InspectRequest) (documentcontract.InspectResponse, error) {
	root := strings.TrimSpace(request.Root)
	if root == "" {
		return documentcontract.InspectResponse{}, fmt.Errorf("document root is required")
	}
	path := strings.TrimSpace(request.Path)
	if path == "" {
		return documentcontract.InspectResponse{}, fmt.Errorf("document path is required")
	}

	resolved, relative, err := e.resolvePath(root, path)
	if err != nil {
		return documentcontract.InspectResponse{}, err
	}
	ext := strings.ToLower(filepath.Ext(resolved))
	if ext != ".md" && ext != ".markdown" {
		return documentcontract.InspectResponse{}, fmt.Errorf("unsupported document format %q", ext)
	}

	source, err := os.ReadFile(resolved)
	if err != nil {
		return documentcontract.InspectResponse{}, fmt.Errorf("read document %s: %w", relative, err)
	}
	lineStarts := sourceLineStarts(source)
	lineCount := len(lineStarts)
	if lineCount == 0 {
		lineCount = 1
	}

	frontmatter, err := parseFrontmatter(source)
	if err != nil {
		return documentcontract.InspectResponse{}, fmt.Errorf("parse frontmatter in %s: %w", relative, err)
	}

	parseSource := maskFrontmatter(source)
	doc := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID())).Parser().Parse(text.NewReader(parseSource))
	flat := collectSections(doc, source, lineStarts, lineCount)
	outline := nestSections(flat)

	response := documentcontract.InspectResponse{
		Path:        filepath.ToSlash(relative),
		Format:      "markdown",
		LineCount:   lineCount,
		Frontmatter: frontmatter,
		Outline:     outline,
	}
	if request.IncludeDiagrams {
		response.Diagrams = collectDiagrams(doc, source, lineStarts)
	}

	selector := strings.TrimSpace(request.Section)
	hasSectionSelection := selector != "" || request.SectionStartLine > 0
	hasRangeSelection := request.RangeStartLine > 0 || request.RangeEndLine > 0
	if hasSectionSelection && hasRangeSelection {
		return documentcontract.InspectResponse{}, fmt.Errorf("document section selection and range selection are mutually exclusive")
	}

	if hasSectionSelection {
		var selected flatSection
		if request.SectionStartLine > 0 {
			selected, err = selectSectionByStartLine(flat, request.SectionStartLine)
		} else {
			selected, err = selectSection(flat, selector)
		}
		if err != nil {
			return documentcontract.InspectResponse{}, err
		}
		result := &documentcontract.SelectedSection{
			Key:       selected.Key,
			Title:     selected.Title,
			Level:     selected.Level,
			StartLine: selected.StartLine,
			EndLine:   selected.EndLine,
		}
		if request.IncludeContent {
			result.Content = contentForLines(source, lineStarts, selected.StartLine, selected.EndLine)
		}
		response.SelectedSection = result
	}

	if hasRangeSelection {
		if request.RangeStartLine <= 0 || request.RangeEndLine <= 0 {
			return documentcontract.InspectResponse{}, fmt.Errorf("document range requires both start and end lines")
		}
		if request.RangeStartLine > request.RangeEndLine {
			return documentcontract.InspectResponse{}, fmt.Errorf("document range start line %d exceeds end line %d", request.RangeStartLine, request.RangeEndLine)
		}
		if request.RangeEndLine > lineCount {
			return documentcontract.InspectResponse{}, fmt.Errorf("document range end line %d exceeds document line count %d", request.RangeEndLine, lineCount)
		}
		result := &documentcontract.SelectedRange{
			StartLine: request.RangeStartLine,
			EndLine:   request.RangeEndLine,
		}
		if request.IncludeContent {
			result.Content = contentForLines(source, lineStarts, request.RangeStartLine, request.RangeEndLine)
		}
		response.SelectedRange = result
	}
	return response, nil
}

func collectDiagrams(root ast.Node, source []byte, starts []int) []documentcontract.Diagram {
	result := make([]documentcontract.Diagram, 0)
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		fence, ok := node.(*ast.FencedCodeBlock)
		if !ok || fence.Info == nil {
			return ast.WalkContinue, nil
		}
		info := strings.Fields(strings.ToLower(strings.TrimSpace(string(fence.Info.Text(source)))))
		if len(info) == 0 || (info[0] != "mermaid" && info[0] != "plantuml" && info[0] != "puml") {
			return ast.WalkContinue, nil
		}
		var content strings.Builder
		endLine := lineForOffset(starts, fence.Info.Segment.Start)
		for i := 0; i < fence.Lines().Len(); i++ {
			segment := fence.Lines().At(i)
			content.Write(segment.Value(source))
			content.WriteByte('\n')
			endLine = lineForOffset(starts, segment.Stop)
		}
		language := info[0]
		if language == "puml" {
			language = "plantuml"
		}
		result = append(result, documentcontract.Diagram{
			Index: len(result) + 1, Language: language,
			StartLine: lineForOffset(starts, fence.Info.Segment.Start),
			EndLine:   endLine + 1, Source: content.String(),
		})
		return ast.WalkContinue, nil
	})
	return result
}

func (e *Extension) resolvePath(root, path string) (string, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve document root: %w", err)
	}

	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(absRoot, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(absRoot, target)
	if err != nil || outside(rel) {
		return "", "", fmt.Errorf("document path %q is outside project root", path)
	}

	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve document path %q: %w", path, err)
	}
	realRel, err := filepath.Rel(realRoot, realTarget)
	if err != nil || outside(realRel) {
		return "", "", fmt.Errorf("document path %q resolves outside project root", path)
	}

	allowed := false
	for _, configuredRoot := range e.roots {
		allowedRoot := configuredRoot
		if !filepath.IsAbs(allowedRoot) {
			allowedRoot = filepath.Join(absRoot, allowedRoot)
		}
		allowedRoot, err = filepath.Abs(allowedRoot)
		if err != nil {
			return "", "", err
		}
		realAllowedRoot, err := filepath.EvalSymlinks(allowedRoot)
		if err != nil {
			return "", "", fmt.Errorf("resolve configured document root %q: %w", configuredRoot, err)
		}
		within, err := filepath.Rel(realAllowedRoot, realTarget)
		if err == nil && !outside(within) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", fmt.Errorf("document path %q is outside configured roots", path)
	}
	return realTarget, rel, nil
}

func outside(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func collectSections(root ast.Node, source []byte, lineStarts []int, lineCount int) []flatSection {
	var sections []flatSection
	_ = ast.Walk(root, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok || heading.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		start := heading.Lines().At(0).Start
		title := strings.TrimSpace(headingText(heading, source))
		key := ""
		if match := numberedHeading.FindStringSubmatch(title); len(match) == 2 {
			key = match[1]
		}
		sections = append(sections, flatSection{
			Key:       key,
			Title:     title,
			Level:     heading.Level,
			StartLine: lineForOffset(lineStarts, start),
			EndLine:   lineCount,
		})
		return ast.WalkContinue, nil
	})
	for i := range sections {
		for j := i + 1; j < len(sections); j++ {
			if sections[j].Level <= sections[i].Level {
				sections[i].EndLine = sections[j].StartLine - 1
				break
			}
		}
	}
	return sections
}

func headingText(heading *ast.Heading, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(heading, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node == heading {
			return ast.WalkContinue, nil
		}
		switch value := node.(type) {
		case *ast.Text:
			b.Write(value.Text(source))
		case *ast.String:
			b.Write(value.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func nestSections(flat []flatSection) []documentcontract.Section {
	index := 0
	var build func(parentLevel int) []documentcontract.Section
	build = func(parentLevel int) []documentcontract.Section {
		var result []documentcontract.Section
		for index < len(flat) {
			current := flat[index]
			if current.Level <= parentLevel {
				break
			}
			index++
			section := documentcontract.Section{
				Key:       current.Key,
				Title:     current.Title,
				Level:     current.Level,
				StartLine: current.StartLine,
				EndLine:   current.EndLine,
			}
			section.Children = build(current.Level)
			result = append(result, section)
		}
		return result
	}
	return build(0)
}

func selectSection(sections []flatSection, selector string) (flatSection, error) {
	var matches []flatSection
	for _, section := range sections {
		if section.Key == selector || strings.EqualFold(section.Title, selector) {
			matches = append(matches, section)
		}
	}
	if len(matches) == 0 {
		for _, section := range sections {
			if strings.HasPrefix(section.Title, selector+" ") || strings.HasPrefix(section.Title, selector+".") {
				matches = append(matches, section)
			}
		}
	}
	switch len(matches) {
	case 0:
		return flatSection{}, fmt.Errorf("document section %q was not found", selector)
	case 1:
		return matches[0], nil
	default:
		return flatSection{}, fmt.Errorf("document section %q is ambiguous", selector)
	}
}

func selectSectionByStartLine(sections []flatSection, startLine int) (flatSection, error) {
	if startLine <= 0 {
		return flatSection{}, fmt.Errorf("document section start line must be positive")
	}
	for _, section := range sections {
		if section.StartLine == startLine {
			return section, nil
		}
	}
	return flatSection{}, fmt.Errorf("document section starting at line %d was not found", startLine)
}

func sourceLineStarts(source []byte) []int {
	starts := []int{0}
	for index, value := range source {
		if value == '\n' && index+1 < len(source) {
			starts = append(starts, index+1)
		}
	}
	return starts
}

func lineForOffset(starts []int, offset int) int {
	line := 1
	for index, start := range starts {
		if start > offset {
			break
		}
		line = index + 1
	}
	return line
}

func contentForLines(source []byte, starts []int, startLine, endLine int) string {
	if startLine < 1 || endLine < startLine || startLine > len(starts) {
		return ""
	}
	start := starts[startLine-1]
	end := len(source)
	if endLine < len(starts) {
		end = starts[endLine]
	}
	return string(source[start:end])
}

func maskFrontmatter(source []byte) []byte {
	lines := bytes.SplitAfter(source, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return source
	}
	masked := append([]byte(nil), source...)
	offset := 0
	for index, line := range lines {
		offset += len(line)
		if index == 0 {
			continue
		}
		if strings.TrimSpace(string(line)) == "---" {
			for i := 0; i < offset; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
			return masked
		}
	}
	return source
}

func parseFrontmatter(source []byte) (map[string]any, error) {
	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, nil
	}
	closeLine := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			closeLine = index
			break
		}
	}
	if closeLine < 0 {
		return nil, fmt.Errorf("frontmatter closing delimiter is missing")
	}
	var value map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:closeLine], "\n")), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func stringSlice(value any) ([]string, error) {
	switch items := value.(type) {
	case []string:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("roots must contain strings")
			}
			if trimmed := strings.TrimSpace(text); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("roots must be an array of strings")
	}
}
