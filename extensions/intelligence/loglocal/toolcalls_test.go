package loglocal

import "testing"

func TestToolCallTrackerParsesStructuredEventsOnly(t *testing.T) {
	tracker := newCallTracker()
	for _, sample := range []struct{ at, text string }{
		{"2026-10-11T10:00:00Z", `🔧 Received tool call 11111111-1111-1111-1111-111111111111: start_process {"timeout_ms":3000}`},
		{"2026-10-11T10:00:01Z", `🔧 Received tool call 22222222-2222-2222-2222-222222222222: start_process {"timeout_ms":3000}`},
		{"2026-10-11T10:00:01.250Z", "✅ Tool call start_process completed:"},
		{"2026-10-11T10:00:01.750Z", "❌ Tool call start_process failed: connection refused"},
		{"2026-10-11T10:00:02Z", `{"content":[{"type":"text","text":"Received tool call fake; timeout_ms=3000"}]}`},
		{"2026-10-11T10:00:03Z", `🔧 Received tool call 33333333-3333-3333-3333-333333333333: read_process_output {"pid":42}`},
		{"2026-10-11T10:00:03.005Z", "✅ Tool call read_process_output completed:"},
	} {
		tracker.record(normalizedRecord{Time: sample.at, Text: sample.text})
	}
	summary := tracker.result()
	if summary == nil || summary.Correlation != "approximate_fifo_by_tool_name" {
		t.Fatalf("bad summary: %#v", summary)
	}
	if summary.TotalReceived != 3 || summary.TotalCompleted != 2 || summary.TotalFailed != 1 {
		t.Fatalf("untrusted payload counted as remote event: %#v", summary)
	}
	if len(summary.Tools) != 2 || summary.Tools[0].Tool != "start_process" {
		t.Fatalf("unexpected metrics: %#v", summary.Tools)
	}
	first := summary.Tools[0]
	if first.Received != 2 || first.Completed != 1 || first.Failed != 1 ||
		first.DurationSamples != 2 || first.P50MS != 750 || first.P90MS != 1250 || first.MaxMS != 1250 {
		t.Fatalf("incorrect latency summary: %#v", first)
	}
	if got := summary.Tools[1]; got.Tool != "read_process_output" || got.P50MS != 5 {
		t.Fatalf("incorrect read metrics: %#v", got)
	}
}

func TestToolCallTrackerIgnoresPlainLogs(t *testing.T) {
	tracker := newCallTracker()
	tracker.record(normalizedRecord{Time: "2026-10-11T10:00:00Z", Text: "timeout_ms=3000 ERROR returned"})
	if tracker.result() != nil {
		t.Fatal("plain log should not produce structured tool metrics")
	}
}
