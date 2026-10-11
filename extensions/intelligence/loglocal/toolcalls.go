package loglocal

import (
	"regexp"
	"slices"
	"time"

	logintelligence "github.com/thinkerqaq/devtool/sdk/logintelligence"
)

// Desktop Commander currently logs request IDs only on its start lines.
// Matching FIFO completions per tool is useful for aggregate performance but
// not exact under concurrent same-name calls. We expose this limitation.
var (
	remoteReceived = regexp.MustCompile(`^🔧 Received tool call [0-9a-f-]+: ([a-z][a-z0-9_]*)\s`)
	remoteFinished = regexp.MustCompile(`^✅ Tool call ([a-z][a-z0-9_]*) completed:`)
	remoteFailed   = regexp.MustCompile(`^❌ Tool call ([a-z][a-z0-9_]*) failed:`)
)

type callSeries struct {
	started   []time.Time
	durations []int64
	received  int
	completed int
	failed    int
}

type callTracker struct {
	series map[string]*callSeries
}

func newCallTracker() *callTracker { return &callTracker{series: map[string]*callSeries{}} }

func (t *callTracker) record(event normalizedRecord) {
	var name string
	var state string
	switch {
	case remoteReceived.MatchString(event.Text):
		name, state = remoteReceived.FindStringSubmatch(event.Text)[1], "received"
	case remoteFinished.MatchString(event.Text):
		name, state = remoteFinished.FindStringSubmatch(event.Text)[1], "completed"
	case remoteFailed.MatchString(event.Text):
		name, state = remoteFailed.FindStringSubmatch(event.Text)[1], "failed"
	default:
		return
	}
	record, ok := t.series[name]
	if !ok {
		record = &callSeries{}
		t.series[name] = record
	}
	at, _ := time.Parse(time.RFC3339Nano, event.Time)
	if state == "received" {
		record.received++
		record.started = append(record.started, at)
		return
	}
	if state == "failed" {
		record.failed++
	} else {
		record.completed++
	}
	if len(record.started) == 0 {
		return
	}
	first := record.started[0]
	record.started = record.started[1:]
	if first.IsZero() || at.IsZero() || at.Before(first) {
		return
	}
	record.durations = append(record.durations, at.Sub(first).Milliseconds())
}

func (t *callTracker) result() *logintelligence.ToolCallSummary {
	if len(t.series) == 0 {
		return nil
	}
	result := &logintelligence.ToolCallSummary{
		Correlation: "approximate_fifo_by_tool_name",
		Note:        "Elapsed time is journal request-to-completion latency, not subprocess runtime. Concurrent calls of the same tool cannot be correlated exactly: completion logs omit IDs.",
	}
	for name, data := range t.series {
		slices.Sort(data.durations)
		metric := logintelligence.ToolCallMetric{
			Tool: name, Received: data.received, Completed: data.completed, Failed: data.failed,
			DurationSamples: len(data.durations),
		}
		if n := len(data.durations); n != 0 {
			metric.P50MS = data.durations[(n-1)/2]
			metric.P90MS = data.durations[(n*9+9)/10-1]
			metric.MaxMS = data.durations[n-1]
		}
		result.TotalReceived += data.received
		result.TotalCompleted += data.completed
		result.TotalFailed += data.failed
		result.Tools = append(result.Tools, metric)
	}
	slices.SortFunc(result.Tools, func(a, b logintelligence.ToolCallMetric) int {
		if a.Received != b.Received {
			if a.Received > b.Received {
				return -1
			}
			return 1
		}
		if a.Tool < b.Tool {
			return -1
		}
		if a.Tool > b.Tool {
			return 1
		}
		return 0
	})
	return result
}
