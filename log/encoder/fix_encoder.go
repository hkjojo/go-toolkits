package encoder

import (
	"encoding/base64"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

var (
	_fixPool = sync.Pool{New: func() interface{} {
		return &fixEncoder{}
	}}

	_bufferPool = buffer.NewPool()
)

func getFixEncoder() *fixEncoder {
	return _fixPool.Get().(*fixEncoder)
}
func putFixEncoder(enc *fixEncoder) {
	enc.EncoderConfig = nil
	enc.buf = nil
	_fixPool.Put(enc)
}

type fixEncoder struct {
	*zapcore.EncoderConfig
	buf        *buffer.Buffer
	reflectBuf *buffer.Buffer
}

// NewFixEncoder ...
func NewFixEncoder(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return &fixEncoder{
		EncoderConfig: &cfg,
	}
}

func (enc *fixEncoder) clone() *fixEncoder {
	clone := getFixEncoder()
	clone.EncoderConfig = enc.EncoderConfig
	clone.buf = _bufferPool.Get()
	return clone
}

func (enc *fixEncoder) Clone() zapcore.Encoder {
	clone := enc.clone()
	clone.buf.Write(enc.buf.Bytes())
	return clone
}

func (enc *fixEncoder) EncodeEntry(ent zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	final := enc.clone()
	if final.TimeKey != "" {
		final.AppendTime(ent.Time)
		final.buf.AppendByte('\t')
	}

	if final.MessageKey != "" {
		// final.AppendString(enc.MessageKey)
		final.AppendString(ent.Message)
	}

	for i := range fields {
		fields[i].AddTo(final)
	}
	final.buf.AppendByte('\n')
	ret := final.buf
	putFixEncoder(final)
	return ret, nil
}

// AddArray implements ObjectEncoder.
func (enc *fixEncoder) AddArray(key string, v zapcore.ArrayMarshaler) error {
	enc.buf.AppendString(key)
	enc.buf.AppendByte('[')
	err := v.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

// AddObject implements ObjectEncoder.
func (enc *fixEncoder) AddObject(k string, v zapcore.ObjectMarshaler) error {
	enc.buf.AppendString(k)
	return enc.AppendObject(v)
}

// AddBinary implements ObjectEncoder.
func (enc *fixEncoder) AddBinary(k string, v []byte) {
	enc.AddString(k, base64.StdEncoding.EncodeToString(v))
}

// AddByteString implements ObjectEncoder.
func (enc *fixEncoder) AddByteString(k string, v []byte) {
	enc.buf.AppendString(k)
	enc.AppendByteString(v)
}

// AddBool implements ObjectEncoder.
func (enc *fixEncoder) AddBool(k string, v bool) {
	enc.buf.AppendString(k)
	enc.AppendBool(v)
}

// AddDuration implements ObjectEncoder.
func (enc *fixEncoder) AddDuration(k string, v time.Duration) {
	enc.buf.AppendString(k)
	enc.AppendDuration(v)
}

// AddComplex128 implements ObjectEncoder.
func (enc *fixEncoder) AddComplex128(k string, v complex128) {
	enc.buf.AppendString(k)
	enc.AppendComplex128(v)
}

// AddComplex64 implements ObjectEncoder.
func (enc *fixEncoder) AddComplex64(k string, v complex64) {
	enc.buf.AppendString(k)
	enc.AppendComplex64(v)
}

// AddFloat64 implements ObjectEncoder.
func (enc *fixEncoder) AddFloat64(k string, v float64) {
	enc.buf.AppendString(k)
	enc.AppendFloat64(v)
}

// AddFloat32 implements ObjectEncoder.
func (enc *fixEncoder) AddFloat32(k string, v float32) {
	enc.buf.AppendString(k)
	enc.AppendFloat32(v)
}

// AddInt implements ObjectEncoder.
func (enc *fixEncoder) AddInt(k string, v int) {
	enc.buf.AppendString(k)
	enc.AppendInt(v)
}

// AddInt64 implements ObjectEncoder.
func (enc *fixEncoder) AddInt64(k string, v int64) {
	enc.buf.AppendString(k)
	enc.AppendInt64(v)
}

// AddInt32 implements ObjectEncoder.
func (enc *fixEncoder) AddInt32(k string, v int32) {
	enc.buf.AppendString(k)
	enc.AppendInt32(v)
}

// AddInt16 implements ObjectEncoder.
func (enc *fixEncoder) AddInt16(k string, v int16) {
	enc.buf.AppendString(k)
	enc.AppendInt16(v)
}

// AddInt8 implements ObjectEncoder.
func (enc *fixEncoder) AddInt8(k string, v int8) {
	enc.buf.AppendString(k)
	enc.AppendInt8(v)
}

// AddString implements ObjectEncoder.
func (enc *fixEncoder) AddString(k string, v string) {
	enc.buf.AppendString(k)
	enc.AppendString(v)
}

// AddTime implements ObjectEncoder.
func (enc *fixEncoder) AddTime(k string, v time.Time) {
	enc.buf.AppendString(k)
	enc.AppendTime(v)
}

// AddUint implements ObjectEncoder.
func (enc *fixEncoder) AddUint(k string, v uint) {
	enc.buf.AppendString(k)
	enc.AppendUint(v)
}

// AddUint64 implements ObjectEncoder.
func (enc *fixEncoder) AddUint64(k string, v uint64) {
	enc.buf.AppendString(k)
	enc.AppendUint64(v)
}

// AddUint32 implements ObjectEncoder.
func (enc *fixEncoder) AddUint32(k string, v uint32) {
	enc.buf.AppendString(k)
	enc.AppendUint32(v)
}

// AddUint16 implements ObjectEncoder.
func (enc *fixEncoder) AddUint16(k string, v uint16) {
	enc.buf.AppendString(k)
	enc.AppendUint16(v)
}

// AddUint8 implements ObjectEncoder.
func (enc *fixEncoder) AddUint8(k string, v uint8) {
	enc.buf.AppendString(k)
	enc.AppendUint8(v)
}

// AddUintptr implements ObjectEncoder.
func (enc *fixEncoder) AddUintptr(k string, v uintptr) {
	enc.buf.AppendString(k)
	enc.AppendUintptr(v)
}

// AddReflected implements ObjectEncoder.
func (enc *fixEncoder) AddReflected(k string, v interface{}) error {
	enc.buf.AppendString(k)
	enc.AppendString(fmt.Sprintf("%v", v))
	return nil
}

// OpenNamespace implements ObjectEncoder.
func (enc *fixEncoder) OpenNamespace(k string) {
	enc.buf.AppendString(k)
}

func (enc *fixEncoder) AppendByteString(val []byte) {
	enc.buf.Write(val)
}

func (enc *fixEncoder) AppendString(val string) {
	enc.buf.AppendString(val)
}

func (enc *fixEncoder) AppendArray(arr zapcore.ArrayMarshaler) error {
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

func (enc *fixEncoder) AppendBool(val bool) {
	enc.buf.AppendBool(val)
}

func (enc *fixEncoder) AppendDuration(val time.Duration) {
	cur := enc.buf.Len()
	enc.EncodeDuration(val, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

func (enc *fixEncoder) AppendTime(val time.Time) {
	cur := enc.buf.Len()
	enc.EncodeTime(val, enc)
	if cur == enc.buf.Len() {
		// User-supplied EncodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(val.UnixNano())
	}
}

func (enc *fixEncoder) AppendObject(v zapcore.ObjectMarshaler) error {
	err := v.MarshalLogObject(enc)
	return err
}

func (enc *fixEncoder) AppendReflected(value interface{}) error {
	enc.AppendString(fmt.Sprintf("%v", value))
	return nil
}

func (enc *fixEncoder) AppendComplex64(v complex64) { enc.AppendComplex128(complex128(v)) }
func (enc *fixEncoder) AppendFloat64(v float64)     { enc.appendFloat(v, 64) }
func (enc *fixEncoder) AppendFloat32(v float32)     { enc.appendFloat(float64(v), 32) }
func (enc *fixEncoder) AppendInt(v int)             { enc.AppendInt64(int64(v)) }
func (enc *fixEncoder) AppendInt32(v int32)         { enc.AppendInt64(int64(v)) }
func (enc *fixEncoder) AppendInt16(v int16)         { enc.AppendInt64(int64(v)) }
func (enc *fixEncoder) AppendInt8(v int8)           { enc.AppendInt64(int64(v)) }
func (enc *fixEncoder) AppendUint(v uint)           { enc.AppendUint64(uint64(v)) }
func (enc *fixEncoder) AppendUint32(v uint32)       { enc.AppendUint64(uint64(v)) }
func (enc *fixEncoder) AppendUint16(v uint16)       { enc.AppendUint64(uint64(v)) }
func (enc *fixEncoder) AppendUint8(v uint8)         { enc.AppendUint64(uint64(v)) }
func (enc *fixEncoder) AppendUintptr(v uintptr)     { enc.AppendUint64(uint64(v)) }

func (enc *fixEncoder) appendFloat(val float64, bitSize int) {
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`NaN`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`+Inf`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`-Inf`)
	default:
		enc.buf.AppendFloat(val, bitSize)
	}
}

func (enc *fixEncoder) AppendInt64(val int64) {
	enc.buf.AppendInt(val)
}

func (enc *fixEncoder) AppendUint64(val uint64) {
	enc.buf.AppendUint(val)
}

func (enc *fixEncoder) AppendComplex128(val complex128) {
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
