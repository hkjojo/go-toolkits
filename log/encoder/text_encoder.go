package encoder

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	atl "github.com/hkjojo/go-toolkits/apptools"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	textTimeFormat = "2006-01-02T15:04:05.000Z"
)

const SPLIT = "\t"

const (
	SPLIT1 = "->"
	SPLIT2 = "-->"
	SPLIT3 = "--->"
	SPLIT4 = "---->"
)

var (
	_textPool = sync.Pool{New: func() interface{} {
		return &textEncoder{}
	}}
)

func getTextEncoder() *textEncoder {
	return _textPool.Get().(*textEncoder)
}

func putTextEncoder(enc *textEncoder) {
	if enc.reflectBuf != nil {
		enc.reflectBuf.Free()
	}
	enc.EncoderConfig = nil
	enc.buf = nil
	enc.reflectBuf = nil
	enc.reflectEnc = nil
	_textPool.Put(enc)
}

type textEncoder struct {
	*zapcore.EncoderConfig
	buf        *buffer.Buffer
	reflectBuf *buffer.Buffer
	reflectEnc *json.Encoder
}

func NewTextEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &textEncoder{
		EncoderConfig: &cfg,
		buf:           _bufferPool.Get(),
	}
}

func (enc *textEncoder) Clone() zapcore.Encoder {
	clone := enc.clone()
	clone.buf.Write(enc.buf.Bytes())
	return clone
}

func (enc *textEncoder) clone() *textEncoder {
	clone := getTextEncoder()
	clone.EncoderConfig = enc.EncoderConfig
	clone.buf = _bufferPool.Get()
	return clone
}

func (enc *textEncoder) formatHeader(t time.Time, level zapcore.Level) {
	// time
	enc.AppendString(t.UTC().Format(textTimeFormat))
	enc.AppendString(SPLIT)

	// level
	enc.EncodeLevel(level, enc)
	enc.AppendString(SPLIT)
}

// EncodeEntry time->|level-|>module->|source->|msg
func (enc *textEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := enc.clone()

	final.formatHeader(ent.Time, ent.Level)

	for _, field := range fields {
		if field.Key == atl.MetaKey_ENV || field.Key == atl.MetaKey_HOSTNAME || field.Key == atl.MetaKey_SERVICE ||
			field.Key == atl.MetaKey_VERSION || field.Key == atl.MetaKey_INSTANCE || field.Key == atl.MetaKey_CALLER {
			continue
		}
		// replace key when system log occur
		if field.Key == "msg" {
			// append log module
			final.AppendString("System")
			final.AppendString(SPLIT)
			// append log source
			final.AppendString("Server")
			final.AppendString(SPLIT)
			// append msg
			final.AppendString(field.String)
			break
		}
		// append log module
		final.AppendString(field.Key)
		final.AppendString(SPLIT)
		// append log source
		final.AppendString(field.String)
		final.AppendString(SPLIT)
		break
	}

	// message
	if final.MessageKey != "" {
		final.AppendString(ent.Message)
	}

	final.AppendString("\n")

	ret := final.buf
	putTextEncoder(final)
	return ret, nil
}

func (enc *textEncoder) AddArray(key string, val zapcore.ArrayMarshaler) error {
	enc.addKey(key)
	return enc.AppendArray(val)
}

func (enc *textEncoder) AddObject(key string, val zapcore.ObjectMarshaler) error {
	enc.addKey(key)
	return enc.AppendObject(val)
}

func (enc *textEncoder) AddBinary(key string, val []byte) {
	enc.AddString(key, base64.StdEncoding.EncodeToString(val))
}

func (enc *textEncoder) AddByteString(key string, val []byte) {
	enc.addKey(key)
	enc.AppendByteString(val)
}

func (enc *textEncoder) AddComplex128(key string, val complex128) {
	enc.addKey(key)
	enc.AppendComplex128(val)
}

func (enc *textEncoder) AddDuration(key string, val time.Duration) {
	enc.addKey(key)
	enc.AppendDuration(val)
}

func (enc *textEncoder) AddTime(key string, val time.Time) {
	enc.addKey(key)
	enc.AppendTime(val)
}

func (enc *textEncoder) AddUint64(key string, val uint64) {
	enc.addKey(key)
	enc.AppendUint64(val)
}

func (enc *textEncoder) AddReflected(key string, val interface{}) error {
	enc.addKey(key)
	enc.buf.AppendString(fmt.Sprintf("%+v", val))
	return nil
}

func (enc *textEncoder) OpenNamespace(key string) {
	enc.buf.AppendString(key)
}

func (enc *textEncoder) addKey(key string) {
	enc.addSeparator()
	enc.buf.AppendString(key)
	enc.buf.AppendByte(':')
}

func (enc *textEncoder) AddString(key, val string) {
	enc.addKey(key)
	enc.buf.AppendString(val)
}

func (enc *textEncoder) AppendString(val string) {
	enc.buf.AppendString(val)
}

func (enc *textEncoder) AddInt64(key string, val int64) {
	enc.addKey(key)
	enc.AppendInt64(val)
}

func (enc *textEncoder) AddBool(key string, val bool) {
	enc.addKey(key)
	enc.AppendBool(val)
}

func (enc *textEncoder) AddFloat64(key string, val float64) {
	enc.addKey(key)
	enc.AppendFloat64(val)
}

func (enc *textEncoder) appendFloat(val float64, bitSize int) {
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`"NaN"`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`"+Inf"`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`"-Inf"`)
	default:
		enc.buf.AppendFloat(val, bitSize)
	}
}

func (enc *textEncoder) addSeparator() {
	if enc.buf.Len() > 0 {
		enc.buf.AppendByte(' ')
	}
}

func (enc *textEncoder) AppendBool(val bool) {
	enc.buf.AppendBool(val)
}

func (enc *textEncoder) AppendByteString(bytes []byte) {
	enc.buf.Write(bytes)
}

func (enc *textEncoder) AppendComplex128(val complex128) {
	// Cast to a platform-independent, fixed-size type.
	r, i := float64(real(val)), float64(imag(val))
	enc.buf.AppendByte('"')
	// Because we're always in a quoted string, we can use strconv without
	// special-casing NaN and +/-Inf.
	enc.buf.AppendFloat(r, 64)
	enc.buf.AppendByte('+')
	enc.buf.AppendFloat(i, 64)
	enc.buf.AppendByte('i')
	enc.buf.AppendByte('"')
}

func (enc *textEncoder) AddComplex64(k string, v complex64) { enc.AddComplex128(k, complex128(v)) }
func (enc *textEncoder) AddFloat32(key string, val float32) { enc.AddFloat64(key, float64(val)) }
func (enc *textEncoder) AddInt(key string, val int)         { enc.AddInt64(key, int64(val)) }
func (enc *textEncoder) AddInt32(key string, val int32)     { enc.AddInt64(key, int64(val)) }
func (enc *textEncoder) AddInt16(key string, val int16)     { enc.AddInt64(key, int64(val)) }
func (enc *textEncoder) AddInt8(key string, val int8)       { enc.AddInt64(key, int64(val)) }
func (enc *textEncoder) AddUint(key string, val uint)       { enc.AddUint64(key, uint64(val)) }
func (enc *textEncoder) AddUint32(key string, val uint32)   { enc.AddUint64(key, uint64(val)) }
func (enc *textEncoder) AddUint16(key string, val uint16)   { enc.AddUint64(key, uint64(val)) }
func (enc *textEncoder) AddUint8(key string, val uint8)     { enc.AddUint64(key, uint64(val)) }
func (enc *textEncoder) AddUintptr(key string, val uintptr) { enc.AddUint64(key, uint64(val)) }
func (enc *textEncoder) AppendFloat64(v float64)            { enc.appendFloat(v, 64) }
func (enc *textEncoder) AppendFloat32(f float32)            { enc.appendFloat(float64(f), 32) }
func (enc *textEncoder) AppendInt(i int)                    { enc.AppendInt64(int64(i)) }
func (enc *textEncoder) AppendInt32(i int32)                { enc.AppendInt64(int64(i)) }
func (enc *textEncoder) AppendInt16(i int16)                { enc.AppendInt64(int64(i)) }
func (enc *textEncoder) AppendInt8(i int8)                  { enc.AppendInt64(int64(i)) }
func (enc *textEncoder) AppendUint(u uint)                  { enc.AppendUint64(uint64(u)) }
func (enc *textEncoder) AppendUint32(u uint32)              { enc.AppendUint64(uint64(u)) }
func (enc *textEncoder) AppendUint16(u uint16)              { enc.AppendUint64(uint64(u)) }
func (enc *textEncoder) AppendUint8(u uint8)                { enc.AppendUint64(uint64(u)) }
func (enc *textEncoder) AppendUintptr(u uintptr)            { enc.AppendUint64(uint64(u)) }
func (enc *textEncoder) AppendComplex64(c complex64)        { enc.AppendComplex128(complex128(c)) }

func (enc *textEncoder) AppendInt64(val int64) {
	enc.buf.AppendInt(val)
}

func (enc *textEncoder) AppendUint64(u uint64) {
	enc.buf.AppendUint(u)
}

func (enc *textEncoder) AppendDuration(val time.Duration) {
	cur := enc.buf.Len()
	enc.EncodeDuration(val, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

func (enc *textEncoder) AppendTime(t time.Time) {
	cur := enc.buf.Len()
	enc.EncodeTime(t, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(t.UnixNano())
	}
}

func (enc *textEncoder) AppendArray(arr zapcore.ArrayMarshaler) error {
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

func (enc *textEncoder) AppendObject(val zapcore.ObjectMarshaler) error {
	enc.AppendString(fmt.Sprintf("%v", val))
	return nil
}

func (enc *textEncoder) AppendReflected(val interface{}) error {
	enc.AppendString(fmt.Sprintf("%v", val))
	return nil
}
