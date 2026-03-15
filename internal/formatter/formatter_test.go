package formatter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// makeMsg builds a LogMsg directly — skips runtime.Caller so tests are
// stable regardless of file/line changes.
func makeMsg(level logmsg.LogLevel, content string, fields []logmsg.Field) *logmsg.LogMsg {
	return &logmsg.LogMsg{
		Timestamp: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Level:     level,
		Content:   content,
		Fields:    fields,
		File:      "app.go",
		Line:      99,
	}
}

// ---------------------------------------------------------------------------
// TextFormatter
// ---------------------------------------------------------------------------

func TestTextFormatter_BasicShape(t *testing.T) {
	f := NewTextFormatter()
	msg := makeMsg(logmsg.INFO, "server started", nil)

	out := f.Format(msg)

	// Must start with [info]
	if !strings.HasPrefix(out, "[info]") {
		t.Errorf("want prefix [info], got %q", out)
	}
	// Must contain the timestamp
	if !strings.Contains(out, "2024-06-01 12:00:00") {
		t.Errorf("timestamp missing in %q", out)
	}
	// Must contain file:line
	if !strings.Contains(out, "app.go:99") {
		t.Errorf("caller missing in %q", out)
	}
	// Must contain message
	if !strings.Contains(out, "server started") {
		t.Errorf("content missing in %q", out)
	}
}

func TestTextFormatter_AllLevels(t *testing.T) {
	f := NewTextFormatter()
	levels := []struct {
		level logmsg.LogLevel
		label string
	}{
		{logmsg.DEBUG, "debug"},
		{logmsg.INFO, "info"},
		{logmsg.WARNING, "warning"},
		{logmsg.ERROR, "error"},
		{logmsg.FATAL, "fatal"},
	}
	for _, tc := range levels {
		out := f.Format(makeMsg(tc.level, "msg", nil))
		want := "[" + tc.label + "]"
		if !strings.HasPrefix(out, want) {
			t.Errorf("level %s: want prefix %q, got %q", tc.label, want, out)
		}
	}
}

func TestTextFormatter_WithFields(t *testing.T) {
	f := NewTextFormatter()
	fields := []logmsg.Field{
		logmsg.StringField("env", "prod"),
		logmsg.IntField("port", 8080),
		logmsg.BoolField("tls", true),
	}
	out := f.Format(makeMsg(logmsg.INFO, "ready", fields))

	if !strings.Contains(out, "| env=\"prod\"") {
		t.Errorf("string field missing in %q", out)
	}
	if !strings.Contains(out, "| port=8080") {
		t.Errorf("int field missing in %q", out)
	}
	if !strings.Contains(out, "| tls=true") {
		t.Errorf("bool field missing in %q", out)
	}
}

func TestTextFormatter_NoFields_NoSeparator(t *testing.T) {
	f := NewTextFormatter()
	out := f.Format(makeMsg(logmsg.INFO, "clean", nil))
	if strings.Contains(out, " | ") {
		t.Errorf("unexpected field separator in no-field message: %q", out)
	}
}

func TestTextFormatter_SpecialCharsInContent(t *testing.T) {
	f := NewTextFormatter()
	out := f.Format(makeMsg(logmsg.ERROR, `can't connect: "db" failed`, nil))
	if !strings.Contains(out, `can't connect: "db" failed`) {
		t.Errorf("special chars in content not preserved: %q", out)
	}
}

func TestTextFormatter_ConcurrentSafe(t *testing.T) {
	// sync.Pool usage must be safe under concurrent calls
	f := NewTextFormatter()
	msg := makeMsg(logmsg.INFO, "concurrent", nil)

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = f.Format(msg)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// JsonFormatter
// ---------------------------------------------------------------------------

func TestJsonFormatter_ValidJSON(t *testing.T) {
	f := NewJsonFormatter()
	out := f.Format(makeMsg(logmsg.INFO, "hello", nil))

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %q", err, out)
	}
}

func TestJsonFormatter_RequiredKeys(t *testing.T) {
	f := NewJsonFormatter()
	out := f.Format(makeMsg(logmsg.WARNING, "low disk", nil))

	var m map[string]any
	json.Unmarshal([]byte(out), &m)

	for _, key := range []string{"timestamp", "level", "caller", "msg"} {
		if _, ok := m[key]; !ok {
			t.Errorf("required key %q missing in JSON output: %q", key, out)
		}
	}
}

func TestJsonFormatter_FieldValues(t *testing.T) {
	f := NewJsonFormatter()
	out := f.Format(makeMsg(logmsg.ERROR, "db failed", nil))

	var m map[string]any
	json.Unmarshal([]byte(out), &m)

	if m["level"] != "error" {
		t.Errorf("level = %v, want error", m["level"])
	}
	if m["msg"] != "db failed" {
		t.Errorf("msg = %v, want \"db failed\"", m["msg"])
	}
	if m["caller"] != "app.go:99" {
		t.Errorf("caller = %v, want app.go:99", m["caller"])
	}
	if m["timestamp"] != "2024-06-01 12:00:00" {
		t.Errorf("timestamp = %v, want 2024-06-01 12:00:00", m["timestamp"])
	}
}

func TestJsonFormatter_WithFields_ValidJSON(t *testing.T) {
	f := NewJsonFormatter()
	fields := []logmsg.Field{
		logmsg.StringField("host", "db01"),
		logmsg.IntField("port", 5432),
		logmsg.BoolField("ssl", true),
		logmsg.Float64Field("latency", 1.23),
	}
	out := f.Format(makeMsg(logmsg.INFO, "connected", fields))

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not valid JSON with fields: %v\noutput: %q", err, out)
	}

	if m["host"] != "db01" {
		t.Errorf("host = %v, want db01", m["host"])
	}
	// JSON numbers decode as float64
	if m["port"] != float64(5432) {
		t.Errorf("port = %v, want 5432", m["port"])
	}
	if m["ssl"] != true {
		t.Errorf("ssl = %v, want true", m["ssl"])
	}
}

func TestJsonFormatter_ContentEscaping(t *testing.T) {
	f := NewJsonFormatter()
	// Quotes in content must be escaped so JSON stays valid
	out := f.Format(makeMsg(logmsg.ERROR, `say "hello"`, nil))

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("JSON invalid after quote escaping: %v\noutput: %q", err, out)
	}
	if m["msg"] != `say "hello"` {
		t.Errorf("msg roundtrip failed: got %v", m["msg"])
	}
}

func TestJsonFormatter_NewlineInContent(t *testing.T) {
	f := NewJsonFormatter()
	out := f.Format(makeMsg(logmsg.INFO, "line1\nline2", nil))

	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("JSON invalid with newline in content: %v\noutput: %q", err, out)
	}
	if m["msg"] != "line1\nline2" {
		t.Errorf("newline not preserved in roundtrip: got %v", m["msg"])
	}
}

func TestJsonFormatter_ConcurrentSafe(t *testing.T) {
	f := NewJsonFormatter()
	msg := makeMsg(logmsg.INFO, "concurrent", nil)

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = f.Format(msg)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 20; i++ {
		<-done
	}
}