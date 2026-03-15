package logmsg

import (
	"fmt"
	"math"
	"strconv"
)

type FieldType int

const (
	_ FieldType = iota
	StringType
	IntType
	Int64Type
	Float64Type
	BoolType
	ErrorType
	AnyType
)

type Field struct {
	Key      string
	Type     FieldType
	StrVal   string  //string ,error,any
	NumVal   int64   // used for Int, Int64, Bool
	FloatVal float64 //float64
}

func StringField(key, val string) Field {
	return Field{
		Key:    key,
		Type:   StringType,
		StrVal: val,
	}
}

func IntField(key string, val int) Field {
	return Field{Key: key, Type: IntType, NumVal: int64(val)}
}

func Int64Field(key string, val int64) Field {
	return Field{Key: key, Type: Int64Type, NumVal: val}
}

func Float64Field(key string, val float64) Field {
	return Field{Key: key, Type: Float64Type, FloatVal: val}
}

func BoolField(key string, val bool) Field {
	n := int64(0)
	if val {
		n = int64(1)
	}
	return Field{
		Key:    key,
		Type:   BoolType,
		NumVal: n,
	}
}

func ErrorField(key string, err error) Field {
	s := ""
	if err != nil {
		s = err.Error()
	}
	return Field{Key: key, Type: ErrorType, StrVal: s}
}

func AnyField(key string, val any) Field {
	return Field{
		Key:    key,
		Type:   AnyType,
		StrVal: fmt.Sprintf("%+v",val),
	}
}

//short hand helper detect type auto

func M(key string,val any) Field{
	switch v:=val.(type){  //type switch,, val.(type) can only be used with switch
	case string:
		return StringField(key, v)
	case int:
		return IntField(key, v)
	case int64:
		return Int64Field(key, v)
	case float64:
		return Float64Field(key, v)
	case bool:
		return BoolField(key, v)
	case error:
		return ErrorField(key, v)
	default:
		return AnyField(key, val)
	}
}


func (f Field) AppendTextValue(buf []byte) []byte {  //for performance
	switch f.Type {
	case StringType, ErrorType, AnyType:
		buf = append(buf, '"')
		buf = append(buf, f.StrVal...)
		return append(buf, '"')
	case IntType, Int64Type:
		return strconv.AppendInt(buf, f.NumVal, 10)
	case Float64Type:
		return strconv.AppendFloat(buf, f.FloatVal, 'f', -1, 64)
	case BoolType:
		if f.NumVal == 1 {
			return append(buf, "true"...)
		}
		return append(buf, "false"...)
	}
	return buf
}

// appendJSONString writes a JSON-escaped string with surrounding quotes.
// Handles the special characters that break JSON parsers.
func AppendJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			if c < 0x20 {  // ADD this — escape all control chars
				buf = append(buf, '\\', 'u', '0', '0')
				buf = append(buf, "0123456789abcdef"[c>>4])
				buf = append(buf, "0123456789abcdef"[c&0xf])
			} else {
				buf = append(buf, c)
			}
		}
	}
	return append(buf, '"')
}


// AppendJSON writes the field as "key":value directly into a byte buffer.
// No allocation — the formatter calls this for every field.
func (f Field) AppendJSON(buf []byte) []byte {
	buf = append(buf, '"')
	buf = append(buf, f.Key...)
	buf = append(buf, '"', ':')
	switch f.Type {
	case StringType, ErrorType, AnyType:
		buf = AppendJSONString(buf, f.StrVal)
	case IntType, Int64Type:
		buf = strconv.AppendInt(buf, f.NumVal, 10)
	case Float64Type:
		if math.IsInf(f.FloatVal, 0) || math.IsNaN(f.FloatVal) {
			buf = append(buf, '"')
			buf = strconv.AppendFloat(buf, f.FloatVal, 'f', -1, 64)
			buf = append(buf, '"')
		} else {
			buf = strconv.AppendFloat(buf, f.FloatVal, 'f', -1, 64)
		}
	case BoolType:
		if f.NumVal == 1 {
			buf = append(buf, "true"...)
		} else {
			buf = append(buf, "false"...)
		}
	}
	return buf
}