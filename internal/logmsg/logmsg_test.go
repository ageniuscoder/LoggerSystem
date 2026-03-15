package logmsg

import (
	"errors"
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseLevel roundtrip
// ---------------------------------------------------------------------------

func TestParseLevel_AllValid(t *testing.T) {
	cases := []struct {
		input string
		want  LogLevel
	}{
		{"debug", DEBUG},
		{"info", INFO},
		{"warning", WARNING},
		{"error", ERROR},
		{"fatal", FATAL},
	}
	for _, tc := range cases {
		got, ok := ParseLevel(tc.input)
		if !ok {
			t.Errorf("ParseLevel(%q) returned ok=false, want true", tc.input)
		}
		if got != tc.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
		// Roundtrip: ToStr must give back the same string
		if got.ToStr() != tc.input {
			t.Errorf("ParseLevel(%q).ToStr() = %q, want %q", tc.input, got.ToStr(), tc.input)
		}
	}
}

func TestParseLevel_Invalid(t *testing.T) {
	bad := []string{"DEBUG", "INFO", "WARN", "warning ", "", "trace", "0"}
	for _, s := range bad {
		_, ok := ParseLevel(s)
		if ok {
			t.Errorf("ParseLevel(%q) returned ok=true, want false", s)
		}
	}
}

func TestLogLevel_Ordering(t *testing.T) {
	// Filtering logic depends on DEBUG < INFO < WARNING < ERROR < FATAL
	if !(DEBUG < INFO && INFO < WARNING && WARNING < ERROR && ERROR < FATAL) {
		t.Error("LogLevel iota ordering is wrong — filtering will be broken")
	}
}

func TestToStr_Unknown(t *testing.T) {
	var bogus LogLevel = 99
	if got := bogus.ToStr(); got != "unknown" {
		t.Errorf("ToStr() on unknown level = %q, want \"unknown\"", got)
	}
}

// ---------------------------------------------------------------------------
// Field constructors
// ---------------------------------------------------------------------------

func TestFieldConstructors_Types(t *testing.T) {
	buf := make([]byte, 0, 128)

	t.Run("string", func(t *testing.T) {
		f := StringField("k", "hello")
		out := string(f.AppendTextValue(buf[:0]))
		if out != `"hello"` {
			t.Errorf("got %q, want %q", out, `"hello"`)
		}
	})

	t.Run("int", func(t *testing.T) {
		f := IntField("k", 42)
		out := string(f.AppendTextValue(buf[:0]))
		if out != "42" {
			t.Errorf("got %q, want %q", out, "42")
		}
	})

	t.Run("int64", func(t *testing.T) {
		f := Int64Field("k", int64(9999999999))
		out := string(f.AppendTextValue(buf[:0]))
		if out != "9999999999" {
			t.Errorf("got %q, want %q", out, "9999999999")
		}
	})

	t.Run("float64", func(t *testing.T) {
		f := Float64Field("k", 3.14)
		out := string(f.AppendTextValue(buf[:0]))
		if !strings.HasPrefix(out, "3.14") {
			t.Errorf("got %q, want prefix 3.14", out)
		}
	})

	t.Run("bool_true", func(t *testing.T) {
		f := BoolField("k", true)
		out := string(f.AppendTextValue(buf[:0]))
		if out != "true" {
			t.Errorf("got %q, want true", out)
		}
	})

	t.Run("bool_false", func(t *testing.T) {
		f := BoolField("k", false)
		out := string(f.AppendTextValue(buf[:0]))
		if out != "false" {
			t.Errorf("got %q, want false", out)
		}
	})

	t.Run("error_non_nil", func(t *testing.T) {
		f := ErrorField("k", errors.New("boom"))
		out := string(f.AppendTextValue(buf[:0]))
		if out != `"boom"` {
			t.Errorf("got %q, want %q", out, `"boom"`)
		}
	})

	t.Run("error_nil", func(t *testing.T) {
		f := ErrorField("k", nil)
		out := string(f.AppendTextValue(buf[:0]))
		if out != `""` {
			t.Errorf("got %q, want empty string field", out)
		}
	})
}

func TestM_TypeDetection(t *testing.T) {
	cases := []struct {
		val  any
		want FieldType
	}{
		{"hello", StringType},
		{42, IntType},
		{int64(1), Int64Type},
		{1.5, Float64Type},
		{true, BoolType},
		{errors.New("e"), ErrorType},
		{struct{}{}, AnyType},
	}
	for _, tc := range cases {
		f := M("k", tc.val)
		if f.Type != tc.want {
			t.Errorf("M(%T) gave FieldType=%v, want %v", tc.val, f.Type, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// AppendJSONString — escape coverage
// ---------------------------------------------------------------------------

func TestAppendJSONString_Escaping(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{`hello`, `"hello"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{"new\nline", `"new\nline"`},
		{"carriage\rreturn", `"carriage\rreturn"`},
		{"tab\there", `"tab\there"`},
		{string([]byte{0x01}), `"\u0001"`},  // control char
		{string([]byte{0x1f}), `"\u001f"`},  // highest control char
	}
	for _, tc := range cases {
		buf := AppendJSONString(nil, tc.input)
		if got := string(buf); got != tc.want {
			t.Errorf("AppendJSONString(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Field.AppendJSON — JSON value encoding
// ---------------------------------------------------------------------------

func TestFieldAppendJSON(t *testing.T) {
	buf := make([]byte, 0, 256)

	t.Run("string_field", func(t *testing.T) {
		f := StringField("host", "db01")
		got := string(f.AppendJSON(buf[:0]))
		if got != `"host":"db01"` {
			t.Errorf("got %q", got)
		}
	})

	t.Run("int_field", func(t *testing.T) {
		f := IntField("port", 5432)
		got := string(f.AppendJSON(buf[:0]))
		if got != `"port":5432` {
			t.Errorf("got %q", got)
		}
	})

	t.Run("bool_field", func(t *testing.T) {
		f := BoolField("ok", false)
		got := string(f.AppendJSON(buf[:0]))
		if got != `"ok":false` {
			t.Errorf("got %q", got)
		}
	})

	t.Run("float_nan_quoted", func(t *testing.T) {
		// NaN must be quoted in JSON output (not valid JSON number)
		f := Float64Field("v", math.NaN())
		got := string(f.AppendJSON(buf[:0]))
		if !strings.Contains(got, `"`) {
			t.Errorf("NaN should be quoted, got %q", got)
		}
	})

	t.Run("float_inf_quoted", func(t *testing.T) {
		f := Float64Field("v", math.Inf(1))
		got := string(f.AppendJSON(buf[:0]))
		if !strings.Contains(got, `"`) {
			t.Errorf("Inf should be quoted, got %q", got)
		}
	})
}

// ---------------------------------------------------------------------------
// NewLogMsg / PutMsgPool
// ---------------------------------------------------------------------------

func TestNewLogMsg_Fields(t *testing.T) {
	fields := []Field{StringField("k", "v")}
	m := NewLogMsg(INFO, "hello world", fields, 1)

	if m.Level != INFO {
		t.Errorf("Level = %v, want INFO", m.Level)
	}
	if m.Content != "hello world" {
		t.Errorf("Content = %q, want %q", m.Content, "hello world")
	}
	if len(m.Fields) != 1 {
		t.Errorf("len(Fields) = %d, want 1", len(m.Fields))
	}
	if m.File == "" {
		t.Error("File should be set by runtime.Caller")
	}
	if m.Line == 0 {
		t.Error("Line should be non-zero")
	}
	if m.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	PutMsgPool(m)
}

func TestPutMsgPool_Zeroes(t *testing.T) {
	m := NewLogMsg(ERROR, "msg", []Field{StringField("a", "b")}, 1)
	PutMsgPool(m)

	// The fields slice is reset, content/file/line cleared
	if m.Content != "" {
		t.Errorf("Content not zeroed after PutMsgPool, got %q", m.Content)
	}
	if m.File != "" {
		t.Errorf("File not zeroed after PutMsgPool, got %q", m.File)
	}
	if m.Line != 0 {
		t.Errorf("Line not zeroed after PutMsgPool, got %d", m.Line)
	}
	if len(m.Fields) != 0 {
		t.Errorf("Fields not zeroed after PutMsgPool, len=%d", len(m.Fields))
	}
}