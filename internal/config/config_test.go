package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// validJSON returns a minimal valid JSON config written to a temp file.
// logPath must use forward slashes (call filepath.ToSlash on Windows).
func writeConfig(t *testing.T, json string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "logger.json")
	if err := os.WriteFile(path, []byte(json), 0644); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

// minimalJSON builds the smallest valid config JSON pointing at logPath.
func minimalJSON(logPath string) string {
	p := filepath.ToSlash(logPath)
	return `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"file","formatter":{"type":"text"},"path":"` + p + `"}
		]}]
	}`
}

// consoleJSON returns a minimal valid config with a console appender (no path needed).
func consoleJSON() string {
	return `{
		"min_level":"debug","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"debug","appenders":[
			{"name":"c","type":"console","formatter":{"type":"text"}}
		]}]
	}`
}

// ---------------------------------------------------------------------------
// Load — file I/O errors
// ---------------------------------------------------------------------------

func TestLoad_NonExistentFile_ReturnsError(t *testing.T) {
	_, err := Load("/no/such/path/logger.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "can't read file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_NoExtension_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noext")
	os.WriteFile(path, []byte(`{}`), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for file with no extension")
	}
	if !strings.Contains(err.Error(), "no extension") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_UnknownExtension_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.WriteFile(path, []byte(`key = "val"`), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !strings.Contains(err.Error(), "parser not exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Load — JSON parsing errors
// ---------------------------------------------------------------------------

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	path := writeConfig(t, `{not valid json`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parser error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_EmptyJSON_ReturnsValidationError(t *testing.T) {
	// {} has no levels — validation must fail
	path := writeConfig(t, `{}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for empty config")
	}
	if !strings.Contains(err.Error(), "validation error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Load — validation: invalid field values
// ---------------------------------------------------------------------------

func TestLoad_InvalidMinLevel_ReturnsValidationError(t *testing.T) {
	path := writeConfig(t, `{
		"min_level":"verbose","min_skip":4,"buffer":100,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for unknown min_level")
	}
}

func TestLoad_BufferExceedsMax_ReturnsValidationError(t *testing.T) {
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":999999,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for buffer > 100000")
	}
}

func TestLoad_BatchSizeZero_DefaultedBeforeValidation(t *testing.T) {
	// batch_size=0 in JSON is treated as "absent" — applyDefaults fills it with
	// 256 before validateStruct runs, so Load must SUCCEED, not fail.
	// This test documents the intentional applyDefaults-before-validate ordering.
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":0,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load should succeed when batch_size=0 (defaulted to 256), got: %v", err)
	}
	if cfg.BatchSize != 256 {
		t.Errorf("BatchSize = %d, want 256 (default)", cfg.BatchSize)
	}
}

func TestLoad_BatchSizeExceedsMax_ReturnsValidationError(t *testing.T) {
	// batch_size > 10000 cannot be defaulted away — validation must catch it.
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":99999,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for batch_size > 10000")
	}
}

func TestLoad_FlushIntervalTooLow_ReturnsValidationError(t *testing.T) {
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":5,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for flush_interval < 10")
	}
}

func TestLoad_MissingPathForFileAppender_ReturnsValidationError(t *testing.T) {
	// file type requires path — omitting it must fail validation
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"file","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for file appender with no path")
	}
}

func TestLoad_UnknownAppenderType_ReturnsValidationError(t *testing.T) {
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"syslog","formatter":{"type":"text"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for unknown appender type")
	}
}

func TestLoad_UnknownFormatterType_ReturnsValidationError(t *testing.T) {
	path := writeConfig(t, `{
		"min_level":"info","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"xml"}}
		]}]
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for unknown formatter type")
	}
}

// ---------------------------------------------------------------------------
// Load — happy path: valid config returns correct values
// ---------------------------------------------------------------------------

func TestLoad_ValidConsoleConfig_ReturnsConfig(t *testing.T) {
	path := writeConfig(t, consoleJSON())

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.MinLevel != "debug" {
		t.Errorf("MinLevel = %q, want debug", cfg.MinLevel)
	}
	if cfg.Buffer != 256 {
		t.Errorf("Buffer = %d, want 256", cfg.Buffer)
	}
	if len(cfg.Levels) != 1 {
		t.Errorf("len(Levels) = %d, want 1", len(cfg.Levels))
	}
}

func TestLoad_ApplyDefaults_FillsZeroValues(t *testing.T) {
	// Provide only required fields; omit buffer/batch_size/flush_interval/min_skip
	// applyDefaults must fill them in before validation runs.
	path := writeConfig(t, `{
		"min_level":"info",
		"levels":[{"level":"info","appenders":[
			{"name":"a","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Buffer == 0 {
		t.Error("Buffer should be defaulted, got 0")
	}
	if cfg.BatchSize == 0 {
		t.Error("BatchSize should be defaulted, got 0")
	}
	if cfg.FlushInterval == 0 {
		t.Error("FlushInterval should be defaulted, got 0")
	}
	if cfg.MinSkip == 0 {
		t.Error("MinSkip should be defaulted, got 0")
	}
}

func TestLoad_AllLevels_ParsedCorrectly(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.ToSlash(filepath.Join(dir, "app.log"))

	levels := []string{"debug", "info", "warning", "error", "fatal"}
	var levelJSON strings.Builder
	for i, lv := range levels {
		if i > 0 {
			levelJSON.WriteString(",")
		}
		levelJSON.WriteString(`{"level":"` + lv + `","appenders":[` +
			`{"name":"` + lv + `","type":"file","formatter":{"type":"text"},"path":"` + logPath + `"}` +
			`]}`)
	}

	path := writeConfig(t, `{
		"min_level":"debug","min_skip":4,"buffer":256,
		"batch_size":10,"flush_interval":50,
		"levels":[`+levelJSON.String()+`]
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Levels) != len(levels) {
		t.Errorf("len(Levels) = %d, want %d", len(cfg.Levels), len(levels))
	}
}

// ---------------------------------------------------------------------------
// applyDefaults — unit tests (internal function, same package)
// ---------------------------------------------------------------------------

func TestApplyDefaults_AllZero(t *testing.T) {
	cfg := &LoggerConfig{}
	applyDefaults(cfg)

	if cfg.Buffer == 0 {
		t.Error("Buffer not defaulted")
	}
	if cfg.MinLevel == "" {
		t.Error("MinLevel not defaulted")
	}
	if cfg.BatchSize == 0 {
		t.Error("BatchSize not defaulted")
	}
	if cfg.FlushInterval == 0 {
		t.Error("FlushInterval not defaulted")
	}
	if cfg.MinSkip == 0 {
		t.Error("MinSkip not defaulted")
	}
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &LoggerConfig{
		Buffer:        512,
		MinLevel:      "error",
		BatchSize:     64,
		FlushInterval: 200,
		MinSkip:       3,
	}
	applyDefaults(cfg)

	if cfg.Buffer != 512 {
		t.Errorf("Buffer was overwritten: got %d", cfg.Buffer)
	}
	if cfg.MinLevel != "error" {
		t.Errorf("MinLevel was overwritten: got %q", cfg.MinLevel)
	}
	if cfg.BatchSize != 64 {
		t.Errorf("BatchSize was overwritten: got %d", cfg.BatchSize)
	}
	if cfg.FlushInterval != 200 {
		t.Errorf("FlushInterval was overwritten: got %d", cfg.FlushInterval)
	}
	if cfg.MinSkip != 3 {
		t.Errorf("MinSkip was overwritten: got %d", cfg.MinSkip)
	}
}

// ---------------------------------------------------------------------------
// applyOptionDefaults — unit tests (internal function, same package)
// ---------------------------------------------------------------------------

func TestApplyOptionDefaults_AllZero(t *testing.T) {
	o := applyOptionDefaults(Options{})

	if o.MinLevel == "" {
		t.Error("MinLevel not defaulted")
	}
	if o.Buffer == 0 {
		t.Error("Buffer not defaulted")
	}
	if o.BatchSize == 0 {
		t.Error("BatchSize not defaulted")
	}
	if o.FlushInterval == 0 {
		t.Error("FlushInterval not defaulted")
	}
	if o.MinSkip == 0 {
		t.Error("MinSkip not defaulted")
	}
}

func TestApplyOptionDefaults_RotatingFileDefaults(t *testing.T) {
	o := applyOptionDefaults(Options{
		MinLevel: "info",
		Appenders: []AppenderOption{
			{Type: "rotating_file", Path: "/tmp/app.log"},
		},
	})

	ao := o.Appenders[0]
	if ao.MaxSize == 0 {
		t.Error("rotating_file MaxSize not defaulted")
	}
	if ao.MaxAge == 0 {
		t.Error("rotating_file MaxAge not defaulted")
	}
	if ao.MaxBackups == 0 {
		t.Error("rotating_file MaxBackups not defaulted")
	}
}

func TestApplyOptionDefaults_FormatterDefault(t *testing.T) {
	o := applyOptionDefaults(Options{
		MinLevel: "info",
		Appenders: []AppenderOption{
			{Type: "console"}, // formatter empty
		},
	})
	if o.Appenders[0].Formatter != "text" {
		t.Errorf("Formatter not defaulted to text, got %q", o.Appenders[0].Formatter)
	}
}

func TestApplyOptionDefaults_PreservesExplicitValues(t *testing.T) {
	o := applyOptionDefaults(Options{
		MinLevel:      "error",
		Buffer:        8192,
		BatchSize:     512,
		FlushInterval: 500,
		MinSkip:       5,
	})
	if o.MinLevel != "error" {
		t.Errorf("MinLevel overwritten: got %q", o.MinLevel)
	}
	if o.Buffer != 8192 {
		t.Errorf("Buffer overwritten: got %d", o.Buffer)
	}
}

// ---------------------------------------------------------------------------
// validateAppenderOption — unit tests (internal function, same package)
// ---------------------------------------------------------------------------

func TestValidateAppenderOption_Console_Valid(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "console", Formatter: "text"})
	if err != nil {
		t.Errorf("console appender should be valid, got: %v", err)
	}
}

func TestValidateAppenderOption_File_RequiresPath(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "file"})
	if err == nil {
		t.Error("file appender with no path should be invalid")
	}
}

func TestValidateAppenderOption_File_WithPath_Valid(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "file", Path: "/tmp/app.log"})
	if err != nil {
		t.Errorf("file appender with path should be valid, got: %v", err)
	}
}

func TestValidateAppenderOption_RotatingFile_RequiresPath(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "rotating_file"})
	if err == nil {
		t.Error("rotating_file with no path should be invalid")
	}
}

func TestValidateAppenderOption_UnknownType_ReturnsError(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "syslog"})
	if err == nil {
		t.Error("unknown type should be invalid")
	}
	if !strings.Contains(err.Error(), "unknown appender type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAppenderOption_UnknownFormatter_ReturnsError(t *testing.T) {
	err := validateAppenderOption(AppenderOption{Type: "console", Formatter: "xml"})
	if err == nil {
		t.Error("unknown formatter should be invalid")
	}
	if !strings.Contains(err.Error(), "unknown formatter") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAppenderOption_EmptyFormatter_IsValid(t *testing.T) {
	// Empty formatter is allowed — applyOptionDefaults fills it in beforehand
	err := validateAppenderOption(AppenderOption{Type: "console", Formatter: ""})
	if err != nil {
		t.Errorf("empty formatter should be valid (defaulted earlier), got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// BuildFromOptions — error paths
// ---------------------------------------------------------------------------

func TestBuildFromOptions_InvalidLevel_ReturnsError(t *testing.T) {
	_, _, err := BuildFromOptions(Options{
		MinLevel:  "TRACE",
		Appenders: []AppenderOption{{Type: "console"}},
	})
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
	if !strings.Contains(err.Error(), "invalid level") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBuildFromOptions_InvalidAppender_ReturnsError_NoGoroutineLeak(t *testing.T) {
	// rotating_file with no path must fail — and must NOT leak a goroutine.
	// We can't directly count goroutines, but we verify the call returns
	// an error and does not block (timeout would fire if goroutine leaked
	// and held a resource).
	done := make(chan error, 1)
	go func() {
		_, _, err := BuildFromOptions(Options{
			MinLevel: "info",
			Appenders: []AppenderOption{
				{Type: "rotating_file"}, // missing path
			},
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error for rotating_file without path")
		}
	case <-time.After(3 * time.Second):
		t.Error("BuildFromOptions blocked for 3s — possible goroutine leak")
	}
}

func TestBuildFromOptions_UnknownAppenderType_ReturnsError(t *testing.T) {
	_, _, err := BuildFromOptions(Options{
		MinLevel:  "info",
		Appenders: []AppenderOption{{Type: "unknown"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown appender type")
	}
}

// ---------------------------------------------------------------------------
// BuildFromOptions — happy path
// ---------------------------------------------------------------------------

func TestBuildFromOptions_Console_ReturnsLogger(t *testing.T) {
	sys, closers, err := BuildFromOptions(Options{
		MinLevel:      "info",
		Buffer:        64,
		BatchSize:     4,
		FlushInterval: 20,
		MinSkip:       4,
		Appenders:     []AppenderOption{{Type: "console", Formatter: "text"}},
	})
	if err != nil {
		t.Fatalf("BuildFromOptions error: %v", err)
	}
	if sys == nil {
		t.Fatal("expected non-nil logger")
	}
	sys.Shutdown()
	for _, c := range closers {
		c()
	}
}

func TestBuildFromOptions_FileAppender_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.log")

	sys, closers, err := BuildFromOptions(Options{
		MinLevel:      "debug",
		Buffer:        64,
		BatchSize:     4,
		FlushInterval: 20,
		MinSkip:       4,
		Appenders:     []AppenderOption{{Type: "file", Path: path, Formatter: "text"}},
	})
	if err != nil {
		t.Fatalf("BuildFromOptions error: %v", err)
	}

	sys.Info("build from options", nil)
	sys.Shutdown()
	for _, c := range closers {
		c()
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "build from options") {
		t.Errorf("message not found in file: %q", string(data))
	}
}

func TestBuildFromOptions_DefaultsApplied_WhenZeroValues(t *testing.T) {
	// Pass an Options with only MinLevel set; all other fields are zero.
	// Must succeed — defaults must be applied before NewLogger is called.
	sys, closers, err := BuildFromOptions(Options{
		MinLevel:  "info",
		Appenders: []AppenderOption{{Type: "console"}},
	})
	if err != nil {
		t.Fatalf("BuildFromOptions with zero values error: %v", err)
	}
	sys.Shutdown()
	for _, c := range closers {
		c()
	}
}

func TestBuildFromOptions_JSONFormatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "json.log")

	sys, closers, err := BuildFromOptions(Options{
		MinLevel:      "info",
		Buffer:        64,
		BatchSize:     4,
		FlushInterval: 20,
		MinSkip:       4,
		Appenders:     []AppenderOption{{Type: "file", Path: path, Formatter: "json"}},
	})
	if err != nil {
		t.Fatalf("BuildFromOptions error: %v", err)
	}

	sys.Info("json test", nil)
	sys.Shutdown()
	for _, c := range closers {
		c()
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"level":"info"`) {
		t.Errorf("JSON output missing level field: %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// Build — happy path via Load + Build pipeline
// ---------------------------------------------------------------------------

func TestBuild_ValidConfig_LogsToFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	cfgPath := writeConfig(t, minimalJSON(logPath))

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	sys, closers, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	sys.Info("via build pipeline", nil)
	sys.Shutdown()
	for _, c := range closers {
		c()
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "via build pipeline") {
		t.Errorf("message not in file: %q", string(data))
	}
}

func TestBuild_InvalidLevel_ReturnsError(t *testing.T) {
	// Bypass Load validation by constructing the struct directly with a bad level.
	cfg := &LoggerConfig{
		MinLevel:      "badlevel",
		MinSkip:       4,
		Buffer:        256,
		BatchSize:     10,
		FlushInterval: 50,
	}
	_, _, err := Build(cfg)
	if err == nil {
		t.Fatal("expected error for invalid level in Build")
	}
}

// ---------------------------------------------------------------------------
// JsonParser
// ---------------------------------------------------------------------------

func TestJsonParser_ValidJSON_ReturnsConfig(t *testing.T) {
	p := &JsonParser{}
	data := []byte(`{
		"min_level":"info","min_skip":4,"buffer":512,
		"batch_size":32,"flush_interval":100,
		"levels":[{"level":"info","appenders":[
			{"name":"c","type":"console","formatter":{"type":"text"}}
		]}]
	}`)

	cfg, err := p.Parse(data)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if cfg.MinLevel != "info" {
		t.Errorf("MinLevel = %q, want info", cfg.MinLevel)
	}
	if cfg.Buffer != 512 {
		t.Errorf("Buffer = %d, want 512", cfg.Buffer)
	}
}

func TestJsonParser_InvalidJSON_ReturnsError(t *testing.T) {
	p := &JsonParser{}
	_, err := p.Parse([]byte(`{not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "json parser") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJsonParser_EmptyBytes_ReturnsError(t *testing.T) {
	p := &JsonParser{}
	_, err := p.Parse([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// ---------------------------------------------------------------------------
// Register / getParser
// ---------------------------------------------------------------------------

func TestRegister_SameExtTwice_FirstWins(t *testing.T) {
	// Register a custom parser for a test-only extension
	ext := "testfmt_" + t.Name() // unique to avoid cross-test interference
	first := &JsonParser{}
	second := &JsonParser{}

	Register(ext, first)
	Register(ext, second) // should be ignored

	got, ok := getParser(ext)
	if !ok {
		t.Fatalf("parser not found for ext %q", ext)
	}
	if got != first {
		t.Error("second Register call overwrote the first — should be first-wins")
	}
}

func TestGetParser_UnknownExt_ReturnsFalse(t *testing.T) {
	_, ok := getParser("nonexistent_extension_xyz")
	if ok {
		t.Error("expected false for unknown extension")
	}
}

func TestGetParser_JSON_ReturnedByInit(t *testing.T) {
	// jsonparser.go registers "json" in init() — must always be present
	p, ok := getParser("json")
	if !ok {
		t.Fatal("json parser not registered — init() may not have run")
	}
	if p == nil {
		t.Fatal("json parser is nil")
	}
}