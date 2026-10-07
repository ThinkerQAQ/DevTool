package content

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	documentcontract "github.com/thinkerqaq/devtool/sdk/document"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	service "github.com/thinkerqaq/devtool/sdk/service"
)

const ExtensionID = "document.relations.content"

type Extension struct {
	articles string
	series   string
	notes    string
	projects string
}

func New() *Extension {
	return &Extension{
		articles: "src/content/articles",
		series:   "src/content/series",
		notes:    "src/content/notes",
		projects: "src/content/projects",
	}
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindDocumentIntelligence,
		Provides: []string{documentcontract.RelationsServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	for key, target := range map[string]*string{
		"articles": &e.articles,
		"series":   &e.series,
		"notes":    &e.notes,
		"projects": &e.projects,
	} {
		if raw, ok := settings[key]; ok {
			value, ok := raw.(string)
			if !ok || strings.TrimSpace(value) == "" {
				return fmt.Errorf("document relation setting %q must be a non-empty string", key)
			}
			*target = strings.TrimSpace(value)
		}
	}
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(documentcontract.RelationsServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(_ context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if method != documentcontract.MethodResolveRelations {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request documentcontract.RelationsRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode document relations request: %w", err)
	}
	response, err := e.resolve(request)
	if err != nil {
		return nil, err
	}
	return json.Marshal(response)
}

type contentDoc struct {
	node      documentcontract.RelationNode
	meta      map[string]any
	category  string
	topic     string
	topicPath []string
	aliases   []string
}

type contentCatalog struct {
	byPath   map[string]*contentDoc
	byKindID map[string][]*contentDoc
	docs     []*contentDoc
}

func (e *Extension) resolve(request documentcontract.RelationsRequest) (documentcontract.RelationsResponse, error) {
	root := strings.TrimSpace(request.Root)
	if root == "" {
		return documentcontract.RelationsResponse{}, fmt.Errorf("document relations root is required")
	}
	path := normalizePath(request.Path)
	if path == "." || path == "" {
		return documentcontract.RelationsResponse{}, fmt.Errorf("document relations path is required")
	}
	depth := request.Depth
	if depth <= 0 {
		depth = 2
	}
	maxNodes := request.MaxNodes
	if maxNodes <= 0 {
		maxNodes = 128
	}

	catalog, err := e.buildCatalog(root)
	if err != nil {
		return documentcontract.RelationsResponse{}, err
	}
	start, ok := catalog.byPath[path]
	if !ok {
		return documentcontract.RelationsResponse{}, fmt.Errorf("document relations path %q was not found in configured content roots", path)
	}

	type queued struct {
		doc   *contentDoc
		depth int
	}
	queue := []queued{{doc: start, depth: 0}}
	seen := map[string]bool{start.node.Key: true}
	nodes := []documentcontract.RelationNode{start.node}
	var edges []documentcontract.RelationEdge
	var warnings []string
	edgeSeen := map[string]bool{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= depth {
			continue
		}
		nextEdges, edgeWarnings := catalog.edgesFor(current.doc)
		warnings = append(warnings, edgeWarnings...)
		for _, edge := range nextEdges {
			edgeKey := edge.Type + "\x00" + edge.From + "\x00" + edge.To + "\x00" + edge.Source
			if !edgeSeen[edgeKey] {
				edgeSeen[edgeKey] = true
				edges = append(edges, edge)
			}
			target := catalog.nodeByKey(edge.To)
			if target == nil || seen[target.node.Key] {
				continue
			}
			if len(nodes) >= maxNodes {
				warnings = appendUnique(warnings, fmt.Sprintf("relation graph truncated at max_nodes=%d", maxNodes))
				continue
			}
			seen[target.node.Key] = true
			nodes = append(nodes, target.node)
			queue = append(queue, queued{doc: target, depth: current.depth + 1})
		}
	}

	return documentcontract.RelationsResponse{
		RootNode: start.node,
		Nodes:    nodes,
		Edges:    edges,
		Warnings: warnings,
	}, nil
}

func (e *Extension) buildCatalog(root string) (*contentCatalog, error) {
	catalog := &contentCatalog{
		byPath:   map[string]*contentDoc{},
		byKindID: map[string][]*contentDoc{},
	}
	roots := []struct {
		kind string
		path string
		skip func(string) bool
	}{
		{kind: "article", path: e.articles, skip: func(rel string) bool {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			return len(parts) > 1 && parts[0] == "en"
		}},
		{kind: "series", path: e.series},
		{kind: "note", path: e.notes},
		{kind: "project", path: e.projects},
	}
	for _, configured := range roots {
		base, err := resolveConfiguredRoot(root, configured.path)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(base); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		err = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".md" && ext != ".markdown" {
				return nil
			}
			within, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			if configured.skip != nil && configured.skip(within) {
				return nil
			}
			doc, err := loadContentDoc(root, path, configured.kind)
			if err != nil {
				return err
			}
			catalog.add(doc)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan %s documents: %w", configured.kind, err)
		}
	}
	sort.Slice(catalog.docs, func(i, j int) bool {
		return catalog.docs[i].node.Path < catalog.docs[j].node.Path
	})
	return catalog, nil
}

func (c *contentCatalog) add(doc *contentDoc) {
	c.docs = append(c.docs, doc)
	c.byPath[doc.node.Path] = doc
	key := doc.node.Kind + ":" + normalizeID(doc.node.ID)
	c.byKindID[key] = append(c.byKindID[key], doc)
}

func (c *contentCatalog) nodeByKey(key string) *contentDoc {
	for _, doc := range c.docs {
		if doc.node.Key == key {
			return doc
		}
	}
	return nil
}

func (c *contentCatalog) edgesFor(doc *contentDoc) ([]documentcontract.RelationEdge, []string) {
	var edges []documentcontract.RelationEdge
	var warnings []string
	addDirect := func(field, targetKind, edgeType string) {
		for _, id := range stringValues(doc.meta[field]) {
			targets := c.resolveID(targetKind, id)
			if len(targets) == 0 {
				warnings = appendUnique(warnings, fmt.Sprintf("%s %q references missing %s %q via %s", doc.node.Kind, doc.node.ID, targetKind, id, field))
				continue
			}
			if len(targets) > 1 {
				warnings = appendUnique(warnings, fmt.Sprintf("%s %q reference %q via %s is ambiguous", doc.node.Kind, doc.node.ID, id, field))
				continue
			}
			edges = append(edges, documentcontract.RelationEdge{
				Type:   edgeType,
				From:   doc.node.Key,
				To:     targets[0].node.Key,
				Source: field,
			})
		}
	}

	switch doc.node.Kind {
	case "article":
		addDirect("series", "series", "member_of_series")
		addDirect("project", "project", "member_of_project")
		addDirect("relatedNotes", "note", "related_note")
	case "series":
		for index, id := range stringValues(doc.meta["relatedArticles"]) {
			targets := c.resolveID("article", id)
			if len(targets) != 1 {
				if len(targets) == 0 {
					warnings = appendUnique(warnings, fmt.Sprintf("series %q references missing article %q", doc.node.ID, id))
				} else {
					warnings = appendUnique(warnings, fmt.Sprintf("series %q article %q is ambiguous", doc.node.ID, id))
				}
				continue
			}
			order := index
			edges = append(edges, documentcontract.RelationEdge{
				Type:   "series_article",
				From:   doc.node.Key,
				To:     targets[0].node.Key,
				Order:  &order,
				Source: "relatedArticles",
			})
		}
		addDirect("relatedNotes", "note", "related_note")
		addDirect("project", "project", "member_of_project")

		for _, category := range stringValues(doc.meta["relatedNoteCategories"]) {
			for _, note := range c.notesMatching(category, nil) {
				edges = append(edges, documentcontract.RelationEdge{
					Type:   "related_note",
					From:   doc.node.Key,
					To:     note.node.Key,
					Label:  category,
					Source: "relatedNoteCategories",
				})
			}
		}
		for _, scope := range mapValues(doc.meta["relatedNoteScopes"]) {
			category := stringValue(scope["category"])
			path := stringValues(scope["topicPath"])
			label := stringValue(scope["label"])
			if label == "" {
				label = strings.Join(append([]string{category}, path...), " · ")
			}
			for _, note := range c.notesMatching(category, path) {
				edges = append(edges, documentcontract.RelationEdge{
					Type:   "related_note",
					From:   doc.node.Key,
					To:     note.node.Key,
					Label:  label,
					Source: "relatedNoteScopes",
				})
			}
		}
	case "project":
		addDirect("tutorials", "article", "project_tutorial")
		addDirect("documentation", "article", "project_documentation")
	}
	return edges, warnings
}

func (c *contentCatalog) resolveID(kind, id string) []*contentDoc {
	id = normalizeID(id)
	if id == "" {
		return nil
	}
	if matches := c.byKindID[kind+":"+id]; len(matches) > 0 {
		return matches
	}
	var result []*contentDoc
	for _, doc := range c.docs {
		if doc.node.Kind != kind {
			continue
		}
		for _, alias := range doc.aliases {
			if normalizeID(alias) == id {
				result = append(result, doc)
				break
			}
		}
	}
	return result
}

func (c *contentCatalog) notesMatching(category string, topicPath []string) []*contentDoc {
	category = strings.TrimSpace(category)
	var result []*contentDoc
	for _, doc := range c.docs {
		if doc.node.Kind != "note" {
			continue
		}
		if category != "" && !strings.EqualFold(doc.category, category) {
			continue
		}
		if len(topicPath) > 0 && !prefixFold(doc.topicPath, topicPath) {
			continue
		}
		result = append(result, doc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].node.Path < result[j].node.Path })
	return result
}

func loadContentDoc(root, path, kind string) (*contentDoc, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s document %s: %w", kind, path, err)
	}
	meta, err := parseFrontmatter(source)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	rel = filepath.ToSlash(rel)
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	doc := &contentDoc{
		node: documentcontract.RelationNode{
			Key:      kind + ":" + rel,
			Kind:     kind,
			ID:       id,
			Path:     rel,
			Title:    stringValue(meta["title"]),
			Status:   stringValue(meta["status"]),
			Language: stringValue(meta["language"]),
		},
		meta:     meta,
		category: stringValue(meta["category"]),
		topic:    stringValue(meta["topic"]),
	}
	doc.topicPath = topicPath(meta)
	doc.aliases = []string{id}
	if sourcePath := stringValue(meta["sourcePath"]); sourcePath != "" {
		doc.aliases = append(doc.aliases, sourcePath, strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath)))
	}
	return doc, nil
}

func resolveConfiguredRoot(projectRoot, configured string) (string, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	target := configured
	if !filepath.IsAbs(target) {
		target = filepath.Join(absRoot, target)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absRoot, target)
	if err != nil || outside(rel) {
		return "", fmt.Errorf("configured content root %q is outside project root", configured)
	}
	return target, nil
}

func parseFrontmatter(source []byte) (map[string]any, error) {
	lines := strings.Split(string(source), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return map[string]any{}, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("frontmatter closing delimiter is missing")
	}
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &meta); err != nil {
		return nil, err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

func topicPath(meta map[string]any) []string {
	if raw, ok := meta["topicPath"]; ok {
		var result []string
		switch values := raw.(type) {
		case []any:
			for _, value := range values {
				switch item := value.(type) {
				case string:
					if strings.TrimSpace(item) != "" {
						result = append(result, strings.TrimSpace(item))
					}
				case map[string]any:
					if id := stringValue(item["id"]); id != "" {
						result = append(result, id)
					}
				case map[any]any:
					if id, ok := item["id"]; ok {
						if text := stringValue(id); text != "" {
							result = append(result, text)
						}
					}
				}
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	if topic := stringValue(meta["topic"]); topic != "" {
		return []string{topic}
	}
	return nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringValues(value any) []string {
	switch values := value.(type) {
	case string:
		if text := strings.TrimSpace(values); text != "" {
			return []string{text}
		}
	case []string:
		var result []string
		for _, value := range values {
			if text := strings.TrimSpace(value); text != "" {
				result = append(result, text)
			}
		}
		return result
	case []any:
		var result []string
		for _, value := range values {
			if text := stringValue(value); text != "" {
				result = append(result, text)
			}
		}
		return result
	}
	return nil
}

func mapValues(value any) []map[string]any {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, value := range values {
		switch item := value.(type) {
		case map[string]any:
			result = append(result, item)
		case map[any]any:
			converted := map[string]any{}
			for key, val := range item {
				converted[fmt.Sprint(key)] = val
			}
			result = append(result, converted)
		}
	}
	return result
}

func prefixFold(full, prefix []string) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if !strings.EqualFold(strings.TrimSpace(full[i]), strings.TrimSpace(prefix[i])) {
			return false
		}
	}
	return true
}

func normalizePath(path string) string {
	return filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
}

func normalizeID(value string) string {
	value = filepath.ToSlash(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, filepath.Ext(value))
	return strings.ToLower(strings.Trim(value, "/"))
}

func outside(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
