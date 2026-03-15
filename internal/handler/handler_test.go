package handler

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ageniuscoder/mlog/internal/logmsg"
)

// ---------------------------------------------------------------------------
// Test double: countingAppender
// Records every msg/batch it receives without doing any I/O.
// ---------------------------------------------------------------------------

type countingAppender struct {
	mu   sync.Mutex
	msgs []*logmsg.LogMsg
}

func (c *countingAppender) AppendMsg(msg *logmsg.LogMsg) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, msg)
	c.mu.Unlock()
	return nil
}

func (c *countingAppender) AppendBatch(msgs []*logmsg.LogMsg) error {
	c.mu.Lock()
	c.msgs = append(c.msgs, msgs...)
	c.mu.Unlock()
	return nil
}

func (c *countingAppender) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

// errAppender always returns an error — used to verify error logging doesn't panic.
type errAppender struct{}

func (e *errAppender) AppendMsg(*logmsg.LogMsg) error  { return errFake }
func (e *errAppender) AppendBatch([]*logmsg.LogMsg) error { return errFake }

type fakeErr struct{}

func (f fakeErr) Error() string { return "fake appender error" }

var errFake = fakeErr{}

// makeMsg builds a LogMsg without going through runtime.Caller.
func makeMsg(level logmsg.LogLevel) *logmsg.LogMsg {
	return &logmsg.LogMsg{
		Level:   level,
		Content: level.ToStr() + " message",
	}
}

// makeBatch builds a slice of LogMsg at the given levels, in order.
func makeBatch(levels ...logmsg.LogLevel) []*logmsg.LogMsg {
	msgs := make([]*logmsg.LogMsg, len(levels))
	for i, l := range levels {
		msgs[i] = makeMsg(l)
	}
	return msgs
}

// ---------------------------------------------------------------------------
// DebugHandler
// ---------------------------------------------------------------------------

func TestDebugHandler_HandleLog_OwnLevel_NotifiesAppender(t *testing.T) {
	ca := &countingAppender{}
	h := NewDebugHandler()
	h.AddAppender(ca)

	h.HandleLog(makeMsg(logmsg.DEBUG))

	if ca.count() != 1 {
		t.Errorf("expected 1 message, got %d", ca.count())
	}
}

func TestDebugHandler_HandleLog_OtherLevel_Forwards(t *testing.T) {
	own := &countingAppender{}
	next := &countingAppender{}

	h := NewDebugHandler()
	h.AddAppender(own)

	// Wire a bare InfoHandler as next to absorb the forwarded message
	ih := NewInfoHandler()
	ih.AddAppender(next)
	h.SetNext(ih)

	h.HandleLog(makeMsg(logmsg.INFO))

	if own.count() != 0 {
		t.Errorf("debug handler must not notify for INFO, notified %d times", own.count())
	}
	if next.count() != 1 {
		t.Errorf("INFO message should reach next handler, got %d", next.count())
	}
}

func TestDebugHandler_HandleBatch_SplitsCorrectly(t *testing.T) {
	own := &countingAppender{}
	infoCa := &countingAppender{}
	warnCa := &countingAppender{}

	h := NewDebugHandler()
	h.AddAppender(own)

	// Wire InfoHandler → WarningHandler so forwarded messages land somewhere.
	// DebugHandler splits: mine=[DEBUG,DEBUG], other=[INFO,WARNING] forwarded to ih.
	// InfoHandler splits: mine=[INFO], other=[WARNING] forwarded to wh.
	wh := NewWarningHandler()
	wh.AddAppender(warnCa)

	ih := NewInfoHandler()
	ih.AddAppender(infoCa)
	ih.SetNext(wh)

	h.SetNext(ih)

	batch := makeBatch(logmsg.DEBUG, logmsg.INFO, logmsg.DEBUG, logmsg.WARNING)
	h.HandleBatch(batch)

	if own.count() != 2 {
		t.Errorf("debug appender: got %d, want 2 (only DEBUG msgs)", own.count())
	}
	if infoCa.count() != 1 {
		t.Errorf("info appender: got %d, want 1 (only INFO msg)", infoCa.count())
	}
	if warnCa.count() != 1 {
		t.Errorf("warning appender: got %d, want 1 (only WARNING msg)", warnCa.count())
	}
}

// ---------------------------------------------------------------------------
// InfoHandler
// ---------------------------------------------------------------------------

func TestInfoHandler_HandleLog_OwnLevel(t *testing.T) {
	ca := &countingAppender{}
	h := NewInfoHandler()
	h.AddAppender(ca)

	h.HandleLog(makeMsg(logmsg.INFO))

	if ca.count() != 1 {
		t.Errorf("got %d, want 1", ca.count())
	}
}

func TestInfoHandler_HandleLog_IgnoresDebug(t *testing.T) {
	ca := &countingAppender{}
	h := NewInfoHandler()
	h.AddAppender(ca)
	// No next set — forward to nil should be a no-op

	h.HandleLog(makeMsg(logmsg.DEBUG))

	if ca.count() != 0 {
		t.Errorf("info handler must not handle DEBUG, got %d", ca.count())
	}
}

func TestInfoHandler_HandleBatch_SplitsCorrectly(t *testing.T) {
	own := &countingAppender{}
	warnCa := &countingAppender{}
	errCa := &countingAppender{}

	h := NewInfoHandler()
	h.AddAppender(own)

	// Wire WarningHandler → ErrorHandler so forwarded messages land somewhere.
	// InfoHandler splits: mine=[INFO,INFO], other=[WARNING,ERROR] forwarded to wh.
	// WarningHandler splits: mine=[WARNING], other=[ERROR] forwarded to eh.
	eh := NewErrorHandler()
	eh.AddAppender(errCa)

	wh := NewWarningHandler()
	wh.AddAppender(warnCa)
	wh.SetNext(eh)

	h.SetNext(wh)

	batch := makeBatch(logmsg.INFO, logmsg.INFO, logmsg.WARNING, logmsg.ERROR)
	h.HandleBatch(batch)

	if own.count() != 2 {
		t.Errorf("info appender: got %d, want 2", own.count())
	}
	if warnCa.count() != 1 {
		t.Errorf("warning appender: got %d, want 1 (only WARNING msg)", warnCa.count())
	}
	if errCa.count() != 1 {
		t.Errorf("error appender: got %d, want 1 (only ERROR msg)", errCa.count())
	}
}

// ---------------------------------------------------------------------------
// WarningHandler
// ---------------------------------------------------------------------------

func TestWarningHandler_HandleLog_OwnLevel(t *testing.T) {
	ca := &countingAppender{}
	h := NewWarningHandler()
	h.AddAppender(ca)

	h.HandleLog(makeMsg(logmsg.WARNING))

	if ca.count() != 1 {
		t.Errorf("got %d, want 1", ca.count())
	}
}

func TestWarningHandler_HandleBatch_SplitsCorrectly(t *testing.T) {
	own := &countingAppender{}
	nextCa := &countingAppender{}

	h := NewWarningHandler()
	h.AddAppender(own)

	eh := NewErrorHandler()
	eh.AddAppender(nextCa)
	h.SetNext(eh)

	batch := makeBatch(logmsg.WARNING, logmsg.ERROR, logmsg.WARNING)
	h.HandleBatch(batch)

	if own.count() != 2 {
		t.Errorf("warning appender: got %d, want 2", own.count())
	}
	if nextCa.count() != 1 {
		t.Errorf("next appender: got %d, want 1 (ERROR forwarded)", nextCa.count())
	}
}

// ---------------------------------------------------------------------------
// ErrorHandler
// ---------------------------------------------------------------------------

func TestErrorHandler_HandleLog_OwnLevel(t *testing.T) {
	ca := &countingAppender{}
	h := NewErrorHandler()
	h.AddAppender(ca)

	h.HandleLog(makeMsg(logmsg.ERROR))

	if ca.count() != 1 {
		t.Errorf("got %d, want 1", ca.count())
	}
}

func TestErrorHandler_HandleLog_Forwards_Fatal(t *testing.T) {
	own := &countingAppender{}
	nextCa := &countingAppender{}

	h := NewErrorHandler()
	h.AddAppender(own)

	fh := NewFatalHandler()
	fh.AddAppender(nextCa)
	h.SetNext(fh)

	h.HandleLog(makeMsg(logmsg.FATAL))

	if own.count() != 0 {
		t.Errorf("error handler must not notify for FATAL, got %d", own.count())
	}
	if nextCa.count() != 1 {
		t.Errorf("FATAL must reach fatal handler, got %d", nextCa.count())
	}
}

// ---------------------------------------------------------------------------
// FatalHandler
// ---------------------------------------------------------------------------

func TestFatalHandler_HandleLog_OwnLevel(t *testing.T) {
	ca := &countingAppender{}
	h := NewFatalHandler()
	h.AddAppender(ca)

	h.HandleLog(makeMsg(logmsg.FATAL))

	if ca.count() != 1 {
		t.Errorf("got %d, want 1", ca.count())
	}
}

func TestFatalHandler_HandleBatch_SplitsCorrectly(t *testing.T) {
	own := &countingAppender{}
	h := NewFatalHandler()
	h.AddAppender(own)
	// No next handler — non-FATAL messages are forwarded to nil (no-op)

	batch := makeBatch(logmsg.FATAL, logmsg.FATAL, logmsg.ERROR)
	h.HandleBatch(batch)

	if own.count() != 2 {
		t.Errorf("fatal appender: got %d, want 2", own.count())
	}
}

// ---------------------------------------------------------------------------
// Full chain: Debug → Info → Warning → Error → Fatal
// ---------------------------------------------------------------------------

func TestFullChain_EachLevelReachesCorrectHandler(t *testing.T) {
	debugCa := &countingAppender{}
	infoCa := &countingAppender{}
	warnCa := &countingAppender{}
	errCa := &countingAppender{}
	fatalCa := &countingAppender{}

	dh := NewDebugHandler()
	ih := NewInfoHandler()
	wh := NewWarningHandler()
	eh := NewErrorHandler()
	fh := NewFatalHandler()

	dh.AddAppender(debugCa)
	ih.AddAppender(infoCa)
	wh.AddAppender(warnCa)
	eh.AddAppender(errCa)
	fh.AddAppender(fatalCa)

	dh.SetNext(ih)
	ih.SetNext(wh)
	wh.SetNext(eh)
	eh.SetNext(fh)

	levels := []struct {
		level logmsg.LogLevel
		ca    *countingAppender
	}{
		{logmsg.DEBUG, debugCa},
		{logmsg.INFO, infoCa},
		{logmsg.WARNING, warnCa},
		{logmsg.ERROR, errCa},
		{logmsg.FATAL, fatalCa},
	}

	for _, tc := range levels {
		dh.HandleLog(makeMsg(tc.level))
	}

	for _, tc := range levels {
		if tc.ca.count() != 1 {
			t.Errorf("level %s: appender got %d messages, want 1",
				tc.level.ToStr(), tc.ca.count())
		}
	}
}

func TestFullChain_BatchRouting(t *testing.T) {
	debugCa := &countingAppender{}
	infoCa := &countingAppender{}
	errCa := &countingAppender{}

	dh := NewDebugHandler()
	ih := NewInfoHandler()
	wh := NewWarningHandler()
	eh := NewErrorHandler()
	fh := NewFatalHandler()

	dh.AddAppender(debugCa)
	ih.AddAppender(infoCa)
	eh.AddAppender(errCa)

	dh.SetNext(ih)
	ih.SetNext(wh)
	wh.SetNext(eh)
	eh.SetNext(fh)

	// 2 debug, 3 info, 1 error — warning and fatal get nothing
	batch := makeBatch(
		logmsg.DEBUG, logmsg.INFO, logmsg.DEBUG,
		logmsg.INFO, logmsg.INFO, logmsg.ERROR,
	)
	dh.HandleBatch(batch)

	if debugCa.count() != 2 {
		t.Errorf("debug: got %d, want 2", debugCa.count())
	}
	if infoCa.count() != 3 {
		t.Errorf("info: got %d, want 3", infoCa.count())
	}
	if errCa.count() != 1 {
		t.Errorf("error: got %d, want 1", errCa.count())
	}
}

// ---------------------------------------------------------------------------
// Multiple appenders on one handler
// ---------------------------------------------------------------------------

func TestHandler_MultipleAppenders_AllNotified(t *testing.T) {
	ca1 := &countingAppender{}
	ca2 := &countingAppender{}
	ca3 := &countingAppender{}

	h := NewInfoHandler()
	h.AddAppender(ca1)
	h.AddAppender(ca2)
	h.AddAppender(ca3)

	h.HandleLog(makeMsg(logmsg.INFO))

	for i, ca := range []*countingAppender{ca1, ca2, ca3} {
		if ca.count() != 1 {
			t.Errorf("appender %d: got %d, want 1", i+1, ca.count())
		}
	}
}

// ---------------------------------------------------------------------------
// Forward to nil next — must be a safe no-op
// ---------------------------------------------------------------------------

func TestHandler_ForwardToNilNext_NoOp(t *testing.T) {
	h := NewDebugHandler()
	// SetNext(nil) explicitly
	h.SetNext(nil)
	// Forwarding a non-DEBUG message hits the nil next — must not panic
	h.HandleLog(makeMsg(logmsg.INFO))
	h.HandleBatch(makeBatch(logmsg.INFO, logmsg.ERROR))
}

// ---------------------------------------------------------------------------
// Appender errors don't panic
// ---------------------------------------------------------------------------

func TestHandler_AppenderError_DoesNotPanic(t *testing.T) {
	h := NewErrorHandler()
	h.AddAppender(&errAppender{})

	// Must not panic despite AppendMsg returning an error
	h.HandleLog(makeMsg(logmsg.ERROR))
	h.HandleBatch(makeBatch(logmsg.ERROR, logmsg.ERROR))
}

// ---------------------------------------------------------------------------
// Empty batch — must not call appenders or panic
// ---------------------------------------------------------------------------

func TestHandler_EmptyBatch_NoAppenderCalls(t *testing.T) {
	ca := &countingAppender{}
	h := NewInfoHandler()
	h.AddAppender(ca)

	h.HandleBatch([]*logmsg.LogMsg{})

	if ca.count() != 0 {
		t.Errorf("expected 0 calls for empty batch, got %d", ca.count())
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety — race detector coverage
// ---------------------------------------------------------------------------

func TestHandler_ConcurrentHandleLog_NoRace(t *testing.T) {
	ca := &countingAppender{}
	h := NewInfoHandler()
	h.AddAppender(ca)

	const goroutines = 10
	const each = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < each; j++ {
				h.HandleLog(makeMsg(logmsg.INFO))
			}
		}()
	}
	wg.Wait()

	if got := ca.count(); got != goroutines*each {
		t.Errorf("concurrent: got %d messages, want %d", got, goroutines*each)
	}
}

func TestHandler_ConcurrentAddAppender_NoRace(t *testing.T) {
	h := NewInfoHandler()
	var wg sync.WaitGroup

	// Concurrent AddAppender + HandleLog — should not data-race
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.AddAppender(&countingAppender{})
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.HandleLog(makeMsg(logmsg.INFO))
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// SetNext replaces the next pointer (not appended)
// ---------------------------------------------------------------------------

func TestHandler_SetNext_Replaces(t *testing.T) {
	first := &countingAppender{}
	second := &countingAppender{}

	// Two INFO handlers — only the one wired as next should receive the message
	h1 := NewDebugHandler()

	ih1 := NewInfoHandler()
	ih1.AddAppender(first)

	ih2 := NewInfoHandler()
	ih2.AddAppender(second)

	h1.SetNext(ih1)
	h1.SetNext(ih2) // replaces ih1

	h1.HandleLog(makeMsg(logmsg.INFO))

	if first.count() != 0 {
		t.Errorf("first handler: got %d, want 0 (should have been replaced)", first.count())
	}
	if second.count() != 1 {
		t.Errorf("second handler: got %d, want 1", second.count())
	}
}

// ---------------------------------------------------------------------------
// Handler with no appenders — HandleLog is a no-op (no panic)
// ---------------------------------------------------------------------------

func TestHandler_NoAppenders_NoOp(t *testing.T) {
	h := NewWarningHandler()
	// No appenders added — must not panic
	h.HandleLog(makeMsg(logmsg.WARNING))
	h.HandleBatch(makeBatch(logmsg.WARNING, logmsg.WARNING))
}

// ---------------------------------------------------------------------------
// Atomic correctness: batch split totals equal input size
// ---------------------------------------------------------------------------

func TestHandler_BatchSplit_TotalsAreConserved(t *testing.T) {
	// Every message must end up in exactly one appender — none lost, none doubled.
	var totalReceived int64

	makeCounter := func() *countingAppender { return &countingAppender{} }
	wrap := func(ca *countingAppender) func() int64 {
		return func() int64 { return int64(ca.count()) }
	}

	dCa, iCa, wCa, eCa, fCa := makeCounter(), makeCounter(), makeCounter(), makeCounter(), makeCounter()

	dh, ih, wh, eh, fh := NewDebugHandler(), NewInfoHandler(), NewWarningHandler(), NewErrorHandler(), NewFatalHandler()
	dh.AddAppender(dCa)
	ih.AddAppender(iCa)
	wh.AddAppender(wCa)
	eh.AddAppender(eCa)
	fh.AddAppender(fCa)
	dh.SetNext(ih); ih.SetNext(wh); wh.SetNext(eh); eh.SetNext(fh)

	const n = 50
	batch := make([]*logmsg.LogMsg, n)
	levels := []logmsg.LogLevel{logmsg.DEBUG, logmsg.INFO, logmsg.WARNING, logmsg.ERROR, logmsg.FATAL}
	for i := range batch {
		batch[i] = makeMsg(levels[i%len(levels)])
	}

	dh.HandleBatch(batch)

	counters := []func() int64{wrap(dCa), wrap(iCa), wrap(wCa), wrap(eCa), wrap(fCa)}
	for _, c := range counters {
		atomic.AddInt64(&totalReceived, c())
	}

	if totalReceived != int64(n) {
		t.Errorf("batch split: total received %d, want %d (messages lost or duplicated)", totalReceived, n)
	}

}