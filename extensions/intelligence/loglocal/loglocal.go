package loglocal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
	logintelligence "github.com/thinkerqaq/devtool/sdk/logintelligence"
	service "github.com/thinkerqaq/devtool/sdk/service"
	devtooltrace "github.com/thinkerqaq/devtool/sdk/trace"
)

const ExtensionID = "intelligence.log.local"

const (
	defaultLimit          = 20
	maxLimit              = 100
	maxLineBytes          = 4 * 1024 * 1024
	maxEvidenceBytes      = 2 * 1024
	levelProbeBytes       = 160
	defaultJournalEntries = 20000
	maxJournalEntries     = 100000
)

var (
	levelPattern       = regexp.MustCompile(`(?i)\b(trace|debug|info|warn(?:ing)?|error|fatal|panic)\b`)
	timestampPattern   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\b`)
	numberPattern      = regexp.MustCompile(`\b\d+\b`)
	hexPattern         = regexp.MustCompile(`(?i)\b(?:0x)?[0-9a-f]{8,}\b`)
	uuidPattern        = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	secretPattern      = regexp.MustCompile(`(?i)\b(authorization|token|password|passwd|secret|api[_-]?key)\s*[:=]\s*([^\s,;]+)`)
	bearerPattern      = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	journalUnitPattern = regexp.MustCompile(`^[A-Za-z0-9@_.:-]+$`)
)

type Extension struct{}

func New() *Extension { return &Extension{} }

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindLogIntelligence,
		Provides: []string{logintelligence.ServiceName},
	}
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(logintelligence.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (result json.RawMessage, err error) {
	ctx, span := devtooltrace.Start(ctx, devtooltrace.Attributes{
		Name:         "local-log",
		Layer:        "provider",
		Service:      logintelligence.ServiceName,
		Provider:     ExtensionID,
		Method:       method,
		RequestBytes: len(payload),
	})
	defer func() { span.End(len(result), err) }()

	if method != logintelligence.MethodAnalyze {
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
	var request logintelligence.AnalyzeRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, fmt.Errorf("decode local log analysis request: %w", err)
	}
	response, err := analyze(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.SourceKind == logintelligence.SourceFile {
		if path, err := sourceFilePath(request); err == nil {
			if format, ok := detectLnavFormat(ctx, path); ok {
				response.LnavUsed = true
				if format != "generic_log" || response.Format == "plain" {
					response.Format = format
				}
			}
		}
	}
	return json.Marshal(response)
}

type patternState struct {
	count  int
	sample string
}

type sourceInput struct {
	reader     io.ReadCloser
	kind       string
	label      string
	format     string
	bytes      int64
	maxEntries int
}

type normalizedRecord struct {
	Time          string
	Source        string
	Level         string
	EmbeddedLevel string
	Text          string
}

func analyze(ctx context.Context, request logintelligence.AnalyzeRequest) (logintelligence.AnalyzeResponse, error) {
	source, err := openSource(ctx, request)
	if err != nil {
		return logintelligence.AnalyzeResponse{}, err
	}
	defer source.reader.Close()

	limit := request.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		return logintelligence.AnalyzeResponse{}, fmt.Errorf("log analysis limit must not exceed %d", maxLimit)
	}
	query := strings.ToLower(strings.TrimSpace(request.Query))

	response := logintelligence.AnalyzeResponse{
		Provider:       ExtensionID,
		Source:         source.label,
		SourceKind:     source.kind,
		Format:         source.format,
		Levels:         map[string]int{},
		EmbeddedLevels: map[string]int{},
		Summary: logintelligence.Summary{
			Bytes: source.bytes,
		},
	}

	scanner := bufio.NewScanner(source.reader)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	patterns := map[string]*patternState{}
	firstContentSeen := false
	candidateEvidence := 0
	continuationIndex := -1
	continuationRemaining := 0

	for scanner.Scan() {
		response.Summary.Lines++
		lineNo := int(response.Summary.Lines)
		line := scanner.Text()
		record := parseRecord(source.kind, line)
		trimmed := strings.TrimSpace(record.Text)

		if source.kind == logintelligence.SourceFile && !firstContentSeen && strings.TrimSpace(line) != "" {
			firstContentSeen = true
			if looksJSON(strings.TrimSpace(line)) {
				response.Format = "json"
			}
		}

		recordLevel(&response, record.Level, false)
		recordLevel(&response, record.EmbeddedLevel, true)

		if isErrorLevel(record.Level) || isErrorLevel(record.EmbeddedLevel) {
			recordPattern(patterns, record.Text)
		}

		matchesQuery := query != "" && strings.Contains(strings.ToLower(record.Text), query)
		evidence := logintelligence.Evidence{
			Line:          lineNo,
			Time:          record.Time,
			Source:        record.Source,
			Level:         record.Level,
			EmbeddedLevel: record.EmbeddedLevel,
			Text:          boundedRedact(record.Text),
		}
		if matchesQuery {
			response.Summary.Matched++
			if len(response.Matches) < limit {
				response.Matches = append(response.Matches, evidence)
			}
		}

		if shouldKeepEvidence(record.Level, record.EmbeddedLevel, matchesQuery) {
			candidateEvidence++
			if len(response.Evidence) < limit {
				response.Evidence = append(response.Evidence, evidence)
				continuationIndex = len(response.Evidence) - 1
				continuationRemaining = 4
			} else {
				continuationIndex = -1
				continuationRemaining = 0
			}
		} else if source.kind == logintelligence.SourceFile && continuationIndex >= 0 && continuationRemaining > 0 && isContinuation(line) {
			response.Evidence[continuationIndex].Text = appendBounded(
				response.Evidence[continuationIndex].Text,
				boundedRedact(line),
			)
			continuationRemaining--
		} else if trimmed != "" {
			continuationIndex = -1
			continuationRemaining = 0
		}
	}
	if err := scanner.Err(); err != nil {
		return logintelligence.AnalyzeResponse{}, fmt.Errorf("scan log source: %w", err)
	}

	response.Patterns = topPatterns(patterns, 10)
	response.Summary.Truncated = candidateEvidence > len(response.Evidence)
	if source.kind == logintelligence.SourceJournald && source.maxEntries > 0 && response.Summary.Lines >= int64(source.maxEntries) {
		response.Summary.Truncated = true
	}
	if len(response.Levels) == 0 {
		response.Levels = nil
	}
	if len(response.EmbeddedLevels) == 0 {
		response.EmbeddedLevels = nil
	}
	return response, nil
}

func openSource(ctx context.Context, request logintelligence.AnalyzeRequest) (sourceInput, error) {
	source := request.Source
	if source == nil {
		source = &logintelligence.Source{Kind: logintelligence.SourceFile, Path: request.Path}
	} else if strings.TrimSpace(request.Path) != "" {
		return sourceInput{}, fmt.Errorf("log analysis path and source are mutually exclusive")
	}

	kind := strings.ToLower(strings.TrimSpace(source.Kind))
	switch kind {
	case "", logintelligence.SourceFile:
		path := strings.TrimSpace(source.Path)
		if path == "" {
			path = strings.TrimSpace(request.Path)
		}
		resolved, err := resolvePath(request.Root, path)
		if err != nil {
			return sourceInput{}, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return sourceInput{}, fmt.Errorf("stat log file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return sourceInput{}, fmt.Errorf("log path %q is not a regular file", path)
		}
		file, err := os.Open(resolved)
		if err != nil {
			return sourceInput{}, fmt.Errorf("open log file: %w", err)
		}
		return sourceInput{
			reader: file,
			kind:   logintelligence.SourceFile,
			label:  path,
			format: "plain",
			bytes:  info.Size(),
		}, nil
	case logintelligence.SourceJournald:
		return openJournalSource(ctx, *source)
	default:
		return sourceInput{}, fmt.Errorf("unsupported log source kind %q", source.Kind)
	}
}

func openJournalSource(ctx context.Context, source logintelligence.Source) (sourceInput, error) {
	unit := strings.TrimSpace(source.Unit)
	if unit == "" {
		return sourceInput{}, fmt.Errorf("journald source unit is required")
	}
	if !journalUnitPattern.MatchString(unit) {
		return sourceInput{}, fmt.Errorf("journald source unit %q contains unsupported characters", unit)
	}
	scope := strings.ToLower(strings.TrimSpace(source.Scope))
	if scope == "" {
		scope = "user"
	}
	if scope != "user" && scope != "system" {
		return sourceInput{}, fmt.Errorf("journald source scope must be user or system")
	}
	maxEntries := source.MaxEntries
	if maxEntries <= 0 {
		maxEntries = defaultJournalEntries
	}
	if maxEntries > maxJournalEntries {
		return sourceInput{}, fmt.Errorf("journald max_entries must not exceed %d", maxJournalEntries)
	}

	args := []string{"--no-pager", "--output=json", "--lines", strconv.Itoa(maxEntries), "--unit", unit}
	if scope == "user" {
		args = append([]string{"--user"}, args...)
	}
	if since, err := normalizeSourceTime("since", source.Since); err != nil {
		return sourceInput{}, err
	} else if since != "" {
		args = append(args, "--since", since)
	}
	if until, err := normalizeSourceTime("until", source.Until); err != nil {
		return sourceInput{}, err
	} else if until != "" {
		args = append(args, "--until", until)
	}
	output, err := exec.CommandContext(ctx, "journalctl", args...).CombinedOutput()
	if err != nil {
		return sourceInput{}, fmt.Errorf("read journald unit %s: %w: %s", unit, err, strings.TrimSpace(string(output)))
	}
	return sourceInput{
		reader:     io.NopCloser(bytes.NewReader(output)),
		kind:       logintelligence.SourceJournald,
		label:      fmt.Sprintf("journald:%s:%s", scope, unit),
		format:     "journald-json",
		bytes:      int64(len(output)),
		maxEntries: maxEntries,
	}, nil
}

func normalizeSourceTime(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", fmt.Errorf("journald source %s must be RFC3339: %w", name, err)
	}
	return parsed.Format(time.RFC3339Nano), nil
}

func parseRecord(kind, line string) normalizedRecord {
	if kind != logintelligence.SourceJournald {
		return normalizedRecord{
			Level: detectLevel(line),
			Text:  line,
		}
	}

	var event map[string]any
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return normalizedRecord{Text: line}
	}
	message := journalString(event["MESSAGE"])
	source := firstNonEmpty(
		journalString(event["SYSLOG_IDENTIFIER"]),
		journalString(event["_SYSTEMD_UNIT"]),
		journalString(event["_COMM"]),
	)
	level := journalPriorityLevel(journalString(event["PRIORITY"]))
	embedded := detectLevel(message)
	return normalizedRecord{
		Time:          journalTimestamp(journalString(event["__REALTIME_TIMESTAMP"])),
		Source:        source,
		Level:         level,
		EmbeddedLevel: embedded,
		Text:          message,
	}
}

func journalString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []any:
		var b strings.Builder
		for _, item := range typed {
			number, ok := item.(float64)
			if !ok || number < 0 || number > 255 {
				return ""
			}
			b.WriteByte(byte(number))
		}
		return strings.TrimSpace(b.String())
	default:
		return ""
	}
}

func journalTimestamp(value string) string {
	if value == "" {
		return ""
	}
	micros, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return ""
	}
	return time.UnixMicro(micros).UTC().Format(time.RFC3339Nano)
}

func journalPriorityLevel(value string) string {
	priority, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	switch priority {
	case 0, 1, 2:
		return "fatal"
	case 3:
		return "error"
	case 4:
		return "warn"
	case 5, 6:
		return "info"
	case 7:
		return "debug"
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func recordLevel(response *logintelligence.AnalyzeResponse, level string, embedded bool) {
	if level == "" {
		return
	}
	if embedded {
		response.EmbeddedLevels[level]++
		switch level {
		case "error", "fatal", "panic":
			response.Summary.EmbeddedErrors++
		case "warn":
			response.Summary.EmbeddedWarnings++
		}
		return
	}
	response.Levels[level]++
	switch level {
	case "error", "fatal", "panic":
		response.Summary.Errors++
	case "warn":
		response.Summary.Warnings++
	}
}

func sourceFilePath(request logintelligence.AnalyzeRequest) (string, error) {
	path := strings.TrimSpace(request.Path)
	if request.Source != nil {
		if strings.TrimSpace(request.Source.Kind) != "" && strings.TrimSpace(request.Source.Kind) != logintelligence.SourceFile {
			return "", fmt.Errorf("not a file source")
		}
		path = strings.TrimSpace(request.Source.Path)
	}
	return resolvePath(request.Root, path)
}

func detectLnavFormat(ctx context.Context, path string) (string, bool) {
	executable, err := exec.LookPath("lnav")
	if err != nil {
		return "", false
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := exec.CommandContext(
		queryCtx,
		executable,
		"-n",
		"-N",
		"-q",
		"-c",
		";select log_format, count(*) as count from all_logs group by log_format order by count desc",
		"-c",
		":write-csv-to -",
		path,
	).Output()
	if err != nil {
		return "", false
	}
	format, err := parseLnavFormat(output)
	if err != nil || format == "" {
		return "", false
	}
	return format, true
}

func parseLnavFormat(raw []byte) (string, error) {
	records, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		return "", err
	}
	if len(records) < 2 || len(records[0]) < 1 {
		return "", nil
	}
	formatColumn := -1
	for i, name := range records[0] {
		if strings.TrimSpace(name) == "log_format" {
			formatColumn = i
			break
		}
	}
	if formatColumn < 0 {
		return "", nil
	}
	for _, record := range records[1:] {
		if formatColumn >= len(record) {
			continue
		}
		if format := strings.TrimSpace(record[formatColumn]); format != "" {
			return format, nil
		}
	}
	return "", nil
}

func resolvePath(root, name string) (string, error) {
	root = strings.TrimSpace(root)
	name = strings.TrimSpace(name)
	if root == "" {
		return "", fmt.Errorf("log analysis project root is required")
	}
	if name == "" {
		return "", fmt.Errorf("log analysis path is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve log analysis root: %w", err)
	}
	absoluteRoot, err = filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", fmt.Errorf("resolve log analysis root symlinks: %w", err)
	}
	candidate := name
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(absoluteRoot, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve log path: %w", err)
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve log path symlinks: %w", err)
	}
	rel, err := filepath.Rel(absoluteRoot, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("log path %q is outside project root", name)
	}
	return filepath.Clean(candidate), nil
}

func detectLevel(line string) string {
	probe := line
	if len(probe) > levelProbeBytes {
		probe = probe[:levelProbeBytes]
	}
	match := levelPattern.FindStringSubmatch(probe)
	if len(match) < 2 {
		return ""
	}
	level := strings.ToLower(match[1])
	if level == "warning" {
		return "warn"
	}
	return level
}

func looksJSON(line string) bool {
	if !strings.HasPrefix(line, "{") {
		return false
	}
	var value map[string]any
	return json.Unmarshal([]byte(line), &value) == nil
}

func isContinuation(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
		return true
	}
	for _, prefix := range []string{"at ", "Caused by:", "Traceback ", "File \"", "... ", "goroutine "} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func shouldKeepEvidence(level, embeddedLevel string, query bool) bool {
	if query {
		return true
	}
	return isInterestingLevel(level) || isInterestingLevel(embeddedLevel)
}

func isInterestingLevel(level string) bool {
	switch level {
	case "warn", "error", "fatal", "panic":
		return true
	default:
		return false
	}
}

func isErrorLevel(level string) bool {
	switch level {
	case "error", "fatal", "panic":
		return true
	default:
		return false
	}
}

func recordPattern(patterns map[string]*patternState, line string) {
	normalized := normalizePattern(line)
	state := patterns[normalized]
	if state == nil {
		state = &patternState{sample: boundedRedact(line)}
		patterns[normalized] = state
	}
	state.count++
}

func normalizePattern(line string) string {
	line = redact(strings.TrimSpace(line))
	line = timestampPattern.ReplaceAllString(line, "<ts>")
	line = uuidPattern.ReplaceAllString(line, "<uuid>")
	line = hexPattern.ReplaceAllString(line, "<hex>")
	line = numberPattern.ReplaceAllString(line, "<n>")
	if len(line) > 240 {
		line = line[:240]
	}
	return line
}

func topPatterns(states map[string]*patternState, limit int) []logintelligence.Pattern {
	out := make([]logintelligence.Pattern, 0, len(states))
	for pattern, state := range states {
		out = append(out, logintelligence.Pattern{
			Pattern: pattern,
			Count:   state.count,
			Sample:  state.sample,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Pattern < out[j].Pattern
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func boundedRedact(line string) string {
	return boundText(redact(line), maxEvidenceBytes)
}

func appendBounded(existing, continuation string) string {
	if existing == "" {
		return boundText(continuation, maxEvidenceBytes)
	}
	return boundText(existing+"\n"+continuation, maxEvidenceBytes)
}

func boundText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	const suffix = "...[truncated]"
	if limit <= len(suffix) {
		return value[:limit]
	}
	return value[:limit-len(suffix)] + suffix
}

func redact(line string) string {
	line = secretPattern.ReplaceAllString(line, "$1=[REDACTED]")
	line = bearerPattern.ReplaceAllString(line, "Bearer [REDACTED]")
	return line
}
