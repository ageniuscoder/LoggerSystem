package mlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Default()
// ---------------------------------------------------------------------------

func TestDefault_ReturnsUsableLogger(t *testing.T) {
	log, stop := Default()
	defer stop()

	if log == nil {
		t.Fatal("Default() returned nil logger")
	}
	// Must not panic
	log.Info("default logger ok")
	log.Warning("warning ok")
}

func TestDefault_StopIsIdempotent(t *testing.T) {
	_, stop := Default()
	stop()
	stop() // must not panic or deadlock
}

// ---------------------------------------------------------------------------
// New()
// ---------------------------------------------------------------------------

func TestNew_NoOptions_BehavesLikeDefault(t *testing.T) {
	log, stop := New()
	defer stop()
	if log == nil {
		t.Fatal("New() returned nil")
	}
	log.Info("new no-opts ok")
}

func TestNew_WithLevel_FiltersCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "level.log")

	log, stop := New(
		WithLevel("error"),
		WithFile(path),
	)
	defer stop()

	log.Debug("should be filtered")
	log.Info("should be filtered")
	log.Warning("should be filtered")
	log.Error("should appear")

	stop()

	data, _ := os.ReadFile(path)
	content := string(data)

	if strings.Contains(content, "should be filtered") {
		t.Errorf("filtered messages appeared in output: %q", content)
	}
	if !strings.Contains(content, "should appear") {
		t.Errorf("error message missing from output: %q", content)
	}
}

func TestNew_WithFile_WritesToDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.log")

	log, stop := New(
		WithLevel("debug"),
		WithFile(path),
	)

	log.Debug("debug msg", M("k", "v"))
	log.Info("info msg")
	log.Error("error msg")
	stop() // flushes and closes

	data, _ := os.ReadFile(path)
	content := string(data)

	for _, want := range []string{"debug msg", "info msg", "error msg"} {
		if !strings.Contains(content, want) {
			t.Errorf("%q not found in file output: %q", want, content)
		}
	}
}

func TestNew_WithJSON_ProducesJSONLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "json.log")

	log, stop := New(
		WithFile(path),
		WithJSON(),
	)
	log.Info("json line", M("host", "db01"))
	stop()

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, `"level":"info"`) {
		t.Errorf("JSON level field missing: %q", content)
	}
	if !strings.Contains(content, `"msg":"json line"`) {
		t.Errorf("JSON msg field missing: %q", content)
	}
	if !strings.Contains(content, `"host":"db01"`) {
		t.Errorf("JSON host field missing: %q", content)
	}
}

func TestNew_InvalidLevel_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid level, got none")
		}
	}()
	New(WithLevel("INVALID"))
}

func TestNew_FileAppender_MissingPath_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing file path, got none")
		}
	}()
	New(WithFile(""))
}

// ---------------------------------------------------------------------------
// M() — field helper
// ---------------------------------------------------------------------------

func TestM_StringField(t *testing.T) {
	f := M("key", "value")
	if f.Key != "key" {
		t.Errorf("Key = %q, want key", f.Key)
	}
}

func TestM_IntField(t *testing.T) {
	f := M("port", 8080)
	if f.Key != "port" {
		t.Errorf("Key = %q, want port", f.Key)
	}
}

// ---------------------------------------------------------------------------
// DroppedCount
// ---------------------------------------------------------------------------

func TestDroppedCount_InitiallyZero(t *testing.T) {
	log, stop := New()
	defer stop()
	if n := log.DroppedCount(); n != 0 {
		t.Errorf("DroppedCount() = %d, want 0 on fresh logger", n)
	}
}

func TestDroppedCount_IncrementsUnderLoad(t *testing.T) {
	// buffer=1 makes it easy to overflow
	log, stop := New(
		WithLevel("debug"),
		WithBuffer(1),
		WithBatchSize(10000),     // prevent normal flushes
		WithFlushInterval(60000), // 60s — won't fire during test
	)

	// Flood far more messages than the buffer holds
	for i := 0; i < 500; i++ {
		log.Info("flood")
	}

	dropped := log.DroppedCount()
	stop()

	if dropped == 0 {
		t.Error("expected DroppedCount > 0 when flooding buffer=1 logger, got 0")
	}
}

// ---------------------------------------------------------------------------
// FromFile()
// ---------------------------------------------------------------------------

func TestFromFile_NonExistentFile_ReturnsError(t *testing.T) {
	_, stop, err := FromFile("/no/such/file.json")
	if err == nil {
		stop()
		t.Fatal("expected error for missing config file, got nil")
	}
}

func TestFromFile_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`{not valid json`), 0644)

	_, stop, err := FromFile(path)
	if err == nil {
		stop()
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFromFile_ValidConfig_Works(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	cfgPath := filepath.Join(dir, "logger.json")

	// filepath.ToSlash converts Windows backslashes to forward slashes.
	// Without this, a path like C:\Users\... embedded in a JSON string
	// produces invalid escape sequences (\U, \p, etc.) and JSON parsing fails.
	// Forward slashes work correctly on all platforms including Windows.
	jsonPath := filepath.ToSlash(logPath)

	cfg := `{
		"min_level": "debug",
		"min_skip": 4,
		"buffer": 256,
		"batch_size": 10,
		"flush_interval": 50,
		"levels": [
			{
				"level": "debug",
				"appenders": [{"name":"d","type":"file","formatter":{"type":"text"},"path":"` + jsonPath + `"}]
			},
			{
				"level": "info",
				"appenders": [{"name":"i","type":"file","formatter":{"type":"text"},"path":"` + jsonPath + `"}]
			},
			{
				"level": "warning",
				"appenders": [{"name":"w","type":"file","formatter":{"type":"text"},"path":"` + jsonPath + `"}]
			},
			{
				"level": "error",
				"appenders": [{"name":"e","type":"file","formatter":{"type":"text"},"path":"` + jsonPath + `"}]
			},
			{
				"level": "fatal",
				"appenders": [{"name":"f","type":"file","formatter":{"type":"text"},"path":"` + jsonPath + `"}]
			}
		]
	}`
	os.WriteFile(cfgPath, []byte(cfg), 0644)

	log, stop, err := FromFile(cfgPath)
	if err != nil {
		t.Fatalf("FromFile error: %v", err)
	}

	log.Info("from file config")
	log.Error("error from file config")
	stop()

	data, _ := os.ReadFile(logPath)
	content := string(data)

	if !strings.Contains(content, "from file config") {
		t.Errorf("info message not in file: %q", content)
	}
	if !strings.Contains(content, "error from file config") {
		t.Errorf("error message not in file: %q", content)
	}
}

// ---------------------------------------------------------------------------
// WithRotatingFile
// ---------------------------------------------------------------------------

func TestNew_WithRotatingFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotate.log")

	log, stop := New(
		WithLevel("debug"),
		WithRotatingFile(path, 10, 1, 3),
	)
	log.Info("rotating log entry")
	stop()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("rotating log file was not created")
	}
}

// ---------------------------------------------------------------------------
// All log methods — smoke test no panic
// ---------------------------------------------------------------------------

func TestAllLogMethods_NoPanic(t *testing.T) {
	log, stop := New(WithLevel("debug"))
	defer stop()

	f := M("k", 1)
	log.Debug("debug", f)
	log.Info("info", f)
	log.Warning("warning", f)
	log.Error("error", f)
	log.Fatal("fatal", f)
}

// ---------------------------------------------------------------------------
// WithBatchSize + WithFlushInterval — tuning options accepted
// ---------------------------------------------------------------------------

func TestNew_TuningOptions(t *testing.T) {
	log, stop := New(
		WithBatchSize(1),
		WithFlushInterval(10),
		WithBuffer(128),
	)
	defer stop()
	log.Info("tuning test")
	time.Sleep(30 * time.Millisecond)
}