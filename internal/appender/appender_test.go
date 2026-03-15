package appender

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ageniuscoder/mlog/internal/formatter"
	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// makeMsg builds a minimal LogMsg without going through runtime.Caller.
func makeMsg(level logmsg.LogLevel, content string) *logmsg.LogMsg {
	return &logmsg.LogMsg{
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Level:     level,
		Content:   content,
		File:      "test.go",
		Line:      1,
	}
}

// tmpFile returns a path inside t.TempDir() — cleaned up automatically.
func tmpFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

// readFile slurps a file and returns its content as a string.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile(%q): %v", path, err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// FileAppender — creation
// ---------------------------------------------------------------------------

func TestNewFileAppender_CreatesFile(t *testing.T) {
	path := tmpFile(t, "app.log")

	fa, err := NewFileAppender(path, formatter.NewTextFormatter())
	if err != nil {
		t.Fatalf("NewFileAppender error: %v", err)
	}
	defer fa.CloseFile()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("NewFileAppender did not create the file")
	}
}

func TestNewFileAppender_BadPath_ReturnsError(t *testing.T) {
	// Directory that does not exist — should fail
	_, err := NewFileAppender("/no/such/directory/app.log", formatter.NewTextFormatter())
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// FileAppender — AppendMsg
// ---------------------------------------------------------------------------

func TestFileAppender_AppendMsg_WritesLine(t *testing.T) {
	path := tmpFile(t, "app.log")
	fa, _ := NewFileAppender(path, formatter.NewTextFormatter())
	defer fa.CloseFile()

	msg := makeMsg(logmsg.INFO, "hello appender")
	if err := fa.AppendMsg(msg); err != nil {
		t.Fatalf("AppendMsg error: %v", err)
	}
	fa.CloseFile()

	content := readFile(t, path)
	if !strings.Contains(content, "hello appender") {
		t.Errorf("message not found in file, got: %q", content)
	}
	// Each AppendMsg must end with a newline
	if !strings.HasSuffix(content, "\n") {
		t.Errorf("file does not end with newline: %q", content)
	}
}

func TestFileAppender_AppendMsg_MultipleMessages(t *testing.T) {
	path := tmpFile(t, "app.log")
	fa, _ := NewFileAppender(path, formatter.NewTextFormatter())
	defer fa.CloseFile()

	messages := []string{"first", "second", "third"}
	for _, m := range messages {
		if err := fa.AppendMsg(makeMsg(logmsg.INFO, m)); err != nil {
			t.Fatalf("AppendMsg(%q) error: %v", m, err)
		}
	}
	fa.CloseFile()

	content := readFile(t, path)
	for _, m := range messages {
		if !strings.Contains(content, m) {
			t.Errorf("message %q not found in file", m)
		}
	}
	// Three messages = three newlines
	if count := strings.Count(content, "\n"); count != 3 {
		t.Errorf("expected 3 newlines, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// FileAppender — AppendBatch
// ---------------------------------------------------------------------------

func TestFileAppender_AppendBatch_WritesAllMessages(t *testing.T) {
	path := tmpFile(t, "batch.log")
	fa, _ := NewFileAppender(path, formatter.NewTextFormatter())
	defer fa.CloseFile()

	batch := []*logmsg.LogMsg{
		makeMsg(logmsg.DEBUG, "batch-one"),
		makeMsg(logmsg.INFO, "batch-two"),
		makeMsg(logmsg.ERROR, "batch-three"),
	}
	if err := fa.AppendBatch(batch); err != nil {
		t.Fatalf("AppendBatch error: %v", err)
	}
	fa.CloseFile()

	content := readFile(t, path)
	for _, msg := range batch {
		if !strings.Contains(content, msg.Content) {
			t.Errorf("batch message %q not found in file", msg.Content)
		}
	}
}

func TestFileAppender_AppendBatch_EmptyIsNoOp(t *testing.T) {
	path := tmpFile(t, "empty.log")
	fa, _ := NewFileAppender(path, formatter.NewTextFormatter())
	defer fa.CloseFile()

	if err := fa.AppendBatch(nil); err != nil {
		t.Errorf("AppendBatch(nil) returned error: %v", err)
	}
	if err := fa.AppendBatch([]*logmsg.LogMsg{}); err != nil {
		t.Errorf("AppendBatch([]) returned error: %v", err)
	}

	fa.CloseFile()
	content := readFile(t, path)
	if content != "" {
		t.Errorf("expected empty file after empty batches, got %q", content)
	}
}

// ---------------------------------------------------------------------------
// FileAppender — appends (does not truncate) on re-open
// ---------------------------------------------------------------------------

func TestFileAppender_AppendsOnReopen(t *testing.T) {
	path := tmpFile(t, "reopen.log")

	fa1, _ := NewFileAppender(path, formatter.NewTextFormatter())
	fa1.AppendMsg(makeMsg(logmsg.INFO, "first-session"))
	fa1.CloseFile()

	fa2, _ := NewFileAppender(path, formatter.NewTextFormatter())
	fa2.AppendMsg(makeMsg(logmsg.INFO, "second-session"))
	fa2.CloseFile()

	content := readFile(t, path)
	if !strings.Contains(content, "first-session") {
		t.Error("first-session was truncated on reopen")
	}
	if !strings.Contains(content, "second-session") {
		t.Error("second-session not written on reopen")
	}
}

// ---------------------------------------------------------------------------
// FileAppender — JSON formatter integration
// ---------------------------------------------------------------------------

func TestFileAppender_JSONFormatter(t *testing.T) {
	path := tmpFile(t, "json.log")
	fa, _ := NewFileAppender(path, formatter.NewJsonFormatter())
	defer fa.CloseFile()

	fa.AppendMsg(makeMsg(logmsg.ERROR, "json test"))
	fa.CloseFile()

	content := readFile(t, path)
	if !strings.Contains(content, `"level":"error"`) {
		t.Errorf("JSON output missing level field: %q", content)
	}
	if !strings.Contains(content, `"msg":"json test"`) {
		t.Errorf("JSON output missing msg field: %q", content)
	}
}

// ---------------------------------------------------------------------------
// FileAppender — concurrent safety
// ---------------------------------------------------------------------------

func TestFileAppender_ConcurrentAppendMsg(t *testing.T) {
	path := tmpFile(t, "concurrent.log")
	fa, _ := NewFileAppender(path, formatter.NewTextFormatter())
	defer fa.CloseFile()

	const goroutines = 10
	const msgsEach = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsEach; j++ {
				fa.AppendMsg(makeMsg(logmsg.INFO, "concurrent"))
			}
		}(i)
	}
	wg.Wait()
	fa.CloseFile()

	content := readFile(t, path)
	lineCount := strings.Count(content, "\n")
	if lineCount != goroutines*msgsEach {
		t.Errorf("expected %d lines, got %d (data race may have corrupted writes)",
			goroutines*msgsEach, lineCount)
	}
}