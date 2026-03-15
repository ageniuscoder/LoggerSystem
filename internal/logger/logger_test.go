package logs

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageniuscoder/mlog/internal/appender"
	"github.com/ageniuscoder/mlog/internal/formatter"
	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// countingAppender records how many messages it received.
type countingAppender struct {
	count int64
}

func (c *countingAppender) AppendMsg(*logmsg.LogMsg) error {
	atomic.AddInt64(&c.count, 1)
	return nil
}
func (c *countingAppender) AppendBatch(msgs []*logmsg.LogMsg) error {
	atomic.AddInt64(&c.count, int64(len(msgs)))
	return nil
}
func (c *countingAppender) Count() int64 { return atomic.LoadInt64(&c.count) }

// newTestLogger builds a Logger wired with a countingAppender at all levels.
// buffer=capacity, batchSize=1 so every message flushes immediately (no
// waiting for a timer), flushInterval kept short.
func newTestLogger(buffer, batchSize int, level logmsg.LogLevel) (*Logger, *countingAppender) {
	ca := &countingAppender{}
	l := NewLogger(buffer, level, batchSize, 4, 20*time.Millisecond)
	l.AddAppender("", ca)
	return l, ca
}

// tmpLog returns a temp file path cleaned up by t.Cleanup.
func tmpLog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.log")
}

// ---------------------------------------------------------------------------
// Shutdown — drains buffer without deadlock
// ---------------------------------------------------------------------------

// TestShutdown_DrainsPendingMessages is the core correctness test.
// We fill the buffer, call Shutdown, and assert every message was written.
// If Shutdown deadlocks the test binary will be killed by -timeout.
func TestShutdown_DrainsPendingMessages(t *testing.T) {
	const n = 50
	// batchSize=100 so nothing flushes until Shutdown drains
	l, ca := newTestLogger(n, 100, logmsg.DEBUG)

	for i := 0; i < n; i++ {
		l.Info("message", []logmsg.Field{logmsg.IntField("i", i)})
	}

	l.Shutdown()

	if got := ca.Count(); got != int64(n) {
		t.Errorf("after Shutdown: got %d messages delivered, want %d", got, n)
	}
}

// TestShutdown_Idempotent ensures calling Shutdown twice does not panic or deadlock.
func TestShutdown_Idempotent(t *testing.T) {
	l, _ := newTestLogger(16, 8, logmsg.DEBUG)
	l.Info("hello", nil)
	l.Shutdown()
	l.Shutdown() // must not block or panic
}

// TestShutdown_NoMessagesIsClean verifies a zero-message logger shuts down cleanly.
func TestShutdown_NoMessagesIsClean(t *testing.T) {
	l, ca := newTestLogger(16, 8, logmsg.DEBUG)
	l.Shutdown()
	if ca.Count() != 0 {
		t.Errorf("expected 0 messages, got %d", ca.Count())
	}
}

// TestShutdown_IgnoresNewLogsAfterClose ensures log() is a no-op post-shutdown.
func TestShutdown_IgnoresNewLogsAfterClose(t *testing.T) {
	l, ca := newTestLogger(16, 1, logmsg.DEBUG)
	l.Shutdown()

	// These must not block, panic, or increment the counter
	l.Info("after shutdown", nil)
	l.Error("also after shutdown", nil)

	// Small wait to rule out a delayed delivery
	time.Sleep(30 * time.Millisecond)
	if ca.Count() != 0 {
		t.Errorf("messages delivered after Shutdown: got %d, want 0", ca.Count())
	}
}

// TestShutdown_FlushesPartialBatch verifies that a batch smaller than batchSize
// is flushed when Shutdown is called (via the drain loop, not the ticker).
func TestShutdown_FlushesPartialBatch(t *testing.T) {
	const batchSize = 100
	const sent = 7 // intentionally less than batchSize
	l, ca := newTestLogger(256, batchSize, logmsg.DEBUG)

	for i := 0; i < sent; i++ {
		l.Debug("partial", nil)
	}
	l.Shutdown()

	if got := ca.Count(); got != sent {
		t.Errorf("partial batch: got %d delivered, want %d", got, sent)
	}
}

// ---------------------------------------------------------------------------
// DroppedCount — increments when buffer is full
// ---------------------------------------------------------------------------

// TestDroppedCount_IncrementsOnFullBuffer sends more messages than the buffer
// can hold without any worker draining it (we use a blocking appender to keep
// the worker busy so the channel stays full).
func TestDroppedCount_IncrementsOnFullBuffer(t *testing.T) {
	// buffer=1 so the channel fills after one message
	const bufferCap = 1
	l := NewLogger(bufferCap, logmsg.DEBUG, 1, 4, 10*time.Millisecond)

	// blockingAppender stalls so the batchWorker cannot drain fast enough.
	// We only need it for setup — the worker will eventually unblock but
	// by then we've already overflowed the channel.
	blocked := make(chan struct{})
	block := &blockingAppender{gate: blocked}
	l.AddAppender("", block)

	// Fill the channel — first message will be consumed by worker (blocked),
	// second sits in the channel; any further ones hit the default branch.
	for i := 0; i < 20; i++ {
		l.Info("flood", nil)
	}

	dropped := l.GetDroppedLogsCnt()
	if dropped == 0 {
		t.Error("expected DroppedCount > 0 after flooding a buffer=1 logger, got 0")
	}

	// Unblock worker so Shutdown can drain cleanly
	close(blocked)
	l.Shutdown()
}

// TestDroppedCount_ZeroWhenBufferSufficient verifies normal operation never drops.
func TestDroppedCount_ZeroWhenBufferSufficient(t *testing.T) {
	const n = 20
	l, _ := newTestLogger(1024, 1, logmsg.DEBUG)
	for i := 0; i < n; i++ {
		l.Info("ok", nil)
	}
	l.Shutdown()

	if d := l.GetDroppedLogsCnt(); d != 0 {
		t.Errorf("expected 0 dropped with large buffer, got %d", d)
	}
}

// blockingAppender stalls AppendBatch until the gate channel is closed.
type blockingAppender struct {
	gate <-chan struct{}
}

func (b *blockingAppender) AppendMsg(*logmsg.LogMsg) error {
	<-b.gate
	return nil
}
func (b *blockingAppender) AppendBatch(msgs []*logmsg.LogMsg) error {
	<-b.gate
	return nil
}

// ---------------------------------------------------------------------------
// MinLevel filtering
// ---------------------------------------------------------------------------

func TestMinLevel_FiltersBelow(t *testing.T) {
	l, ca := newTestLogger(64, 1, logmsg.WARNING)

	l.Debug("should be filtered", nil)
	l.Info("should be filtered", nil)
	l.Warning("should pass", nil)
	l.Error("should pass", nil)

	l.Shutdown()

	if got := ca.Count(); got != 2 {
		t.Errorf("minLevel=warning: expected 2 messages, got %d", got)
	}
}

func TestMinLevel_DebugPassesAll(t *testing.T) {
	l, ca := newTestLogger(64, 1, logmsg.DEBUG)

	l.Debug("d", nil)
	l.Info("i", nil)
	l.Warning("w", nil)
	l.Error("e", nil)
	l.Fatal("f", nil)

	l.Shutdown()

	if got := ca.Count(); got != 5 {
		t.Errorf("minLevel=debug: expected 5 messages, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// AddAppender — level-specific vs all-levels
// ---------------------------------------------------------------------------

func TestAddAppender_LevelSpecific(t *testing.T) {
	errorOnly := &countingAppender{}
	l := NewLogger(64, logmsg.DEBUG, 1, 4, 20*time.Millisecond)
	l.AddAppender("error", errorOnly)

	l.Debug("d", nil)
	l.Info("i", nil)
	l.Warning("w", nil)
	l.Error("e", nil) // only this one reaches errorOnly
	l.Shutdown()

	if got := errorOnly.Count(); got != 1 {
		t.Errorf("error-only appender: got %d, want 1", got)
	}
}

func TestAddAppender_AllLevels(t *testing.T) {
	all := &countingAppender{}
	l := NewLogger(64, logmsg.DEBUG, 1, 4, 20*time.Millisecond)
	l.AddAppender("", all)

	l.Debug("d", nil)
	l.Info("i", nil)
	l.Warning("w", nil)
	l.Error("e", nil)
	l.Fatal("f", nil)
	l.Shutdown()

	if got := all.Count(); got != 5 {
		t.Errorf("all-levels appender: got %d, want 5", got)
	}
}

// ---------------------------------------------------------------------------
// File integration — messages actually land on disk
// ---------------------------------------------------------------------------

func TestLogger_FileAppender_MessagesOnDisk(t *testing.T) {
	path := tmpLog(t)
	fa, err := appender.NewFileAppender(path, formatter.NewTextFormatter())
	if err != nil {
		t.Fatalf("NewFileAppender: %v", err)
	}

	l := NewLogger(256, logmsg.DEBUG, 1, 4, 20*time.Millisecond)
	l.AddAppender("", fa)

	l.Info("written to disk", []logmsg.Field{logmsg.StringField("key", "val")})
	l.Error("also written", nil)

	l.Shutdown()
	fa.CloseFile()

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "written to disk") {
		t.Errorf("info message not on disk: %q", content)
	}
	if !strings.Contains(content, "also written") {
		t.Errorf("error message not on disk: %q", content)
	}
	if !strings.Contains(content, "key") {
		t.Errorf("field not on disk: %q", content)
	}
}

// ---------------------------------------------------------------------------
// Concurrent logging — race detector coverage
// ---------------------------------------------------------------------------

func TestLogger_ConcurrentLog_NoRace(t *testing.T) {
	l, _ := newTestLogger(4096, 32, logmsg.DEBUG)

	const goroutines = 8
	const each = 200
	done := make(chan struct{}, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			for i := 0; i < each; i++ {
				l.Info("concurrent", []logmsg.Field{logmsg.IntField("g", id)})
			}
			done <- struct{}{}
		}(g)
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
	l.Shutdown()
}

// ---------------------------------------------------------------------------
// Ticker flush — messages arrive before batchSize is reached
// ---------------------------------------------------------------------------

func TestLogger_TickerFlush(t *testing.T) {
	// batchSize=1000 but we only send 3; the ticker (20ms) must flush them.
	l, ca := newTestLogger(256, 1000, logmsg.DEBUG)

	l.Info("tick1", nil)
	l.Info("tick2", nil)
	l.Info("tick3", nil)

	// Wait longer than flush interval, then shut down
	time.Sleep(60 * time.Millisecond)
	l.Shutdown()

	if got := ca.Count(); got != 3 {
		t.Errorf("ticker flush: got %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// Caller skip — file name in message is not logger internals
// ---------------------------------------------------------------------------

func TestLogger_CallerSkip_PointsToCallSite(t *testing.T) {
	path := tmpLog(t)
	fa, _ := appender.NewFileAppender(path, formatter.NewTextFormatter())

	// skip=4 is the default — should point to this test file
	l := NewLogger(64, logmsg.DEBUG, 1, 4, 20*time.Millisecond)
	l.AddAppender("", fa)
	l.Info("caller test", nil)
	l.Shutdown()
	fa.CloseFile()

	data, _ := os.ReadFile(path)
	content := string(data)

	// The caller should be this test file, not log.go or handle.go
	if strings.Contains(content, "log.go") {
		t.Errorf("caller points to logger internals instead of call site: %q", content)
	}
	// Must contain a .go file reference
	if !strings.Contains(content, ".go:") {
		t.Errorf("no caller info in output: %q", content)
	}
}