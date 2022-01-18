package hook

import (
	"fmt"
	"math"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
)

// Core ..
type Core interface {
	writeData(data *CoreData)
}

// CoreConfig default config
type CoreConfig struct {
	QueueLength uint32            `json:"queue_length"`
	Filter      []string          `json:"filter"`
	Fields      map[string]string `json:"fields"`
	Level       string            `json:"level"`
	Off         bool              `json:"off"`
}

// BaseCore BaseCore
type BaseCore struct {
	zapcore.LevelEnabler

	filters    map[string]bool
	fields     map[string]string
	withfields []zapcore.Field
	queue      chan *CoreData
	core       Core
	enc        zapcore.Encoder
	out        zapcore.WriteSyncer
	off        bool
}

// CoreData ..
type CoreData struct {
	entry  zapcore.Entry
	fields []zapcore.Field
}

// With ..
func (c *BaseCore) With(fields []zapcore.Field) zapcore.Core {
	clone := c.clone()
	clone.addFields(fields)
	return clone
}

// Write ..
func (c *BaseCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	if c.core != nil {
		var fs = SliceFieldToMap(fields)
		for _, field := range c.withfields {
			fs[field.Key] = field
		}

		c.filterFields(fs)
		err := c.write(ent, MapFieldToSlice(fs))
		if err != nil {
			return err
		}
	}

	if c.LevelEnabler.Enabled(ent.Level) {
		c.Sync()
	}

	return nil
}

// Check ..
func (c *BaseCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.off {
		return ce
	}

	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Sync ..
func (c *BaseCore) Sync() error {
	return c.out.Sync()
}

func (c *BaseCore) write(entry zapcore.Entry, fields []zapcore.Field) (err error) {

	select {
	case c.queue <- &CoreData{
		entry:  entry,
		fields: fields,
	}:
	default:
		fmt.Fprintf(os.Stderr, "log channel is full entry[%v] abort [%d]",
			entry, len(c.queue))
		for len(c.queue) != 0 {
			<-c.queue
		}
	}
	return nil
}

func (c *BaseCore) clone() *BaseCore {
	return &BaseCore{
		LevelEnabler: c.LevelEnabler,
		enc:          c.enc.Clone(),
		out:          c.out,
		core:         c.core,
		filters:      c.filters,
		fields:       c.fields,
		queue:        c.queue,
		off:          c.off,
		withfields:   c.withfields,
	}
}

// Start ..
func (c *BaseCore) start() {
	go func() {
		for {
			select {
			case entry := <-c.queue:
				c.writeData(entry)
			}
		}
	}()
}

func (c *BaseCore) filter(key string) bool {
	if c.filters == nil {
		return false
	}

	if _, ok := c.filters[key]; ok {
		return true
	}
	return false
}

func (c *BaseCore) filterFields(fields map[string]zapcore.Field) {
	for k, v := range c.fields {
		fields[k] = zapcore.Field{
			Type:   zapcore.StringType,
			Key:    k,
			String: v,
		}
	}
	for k := range c.filters {
		delete(fields, k)
	}
	return
}

func (c *BaseCore) getFieldString(field zapcore.Field) string {
	return fmt.Sprintf("%v", c.getField(field))
}

func (c *BaseCore) getField(field zapcore.Field) interface{} {
	switch field.Type {
	case zapcore.ArrayMarshalerType,
		zapcore.ObjectMarshalerType,
		zapcore.BinaryType,
		zapcore.ByteStringType,
		zapcore.Complex128Type,
		zapcore.ReflectType,
		zapcore.StringerType,
		zapcore.Complex64Type,
		zapcore.ErrorType:
		return field.Interface
	case zapcore.StringType:
		return field.String
	case zapcore.TimeType:
		if field.Interface != nil {
			return time.Unix(0, field.Integer).In(field.Interface.(*time.Location))
		}
		return time.Unix(0, field.Integer)
	case zapcore.Float64Type:
		return math.Float64frombits(uint64(field.Integer))
	case zapcore.Float32Type:
		return math.Float32frombits(uint32(field.Integer))
	}

	return field.Integer
}

func (c *BaseCore) writeData(data *CoreData) {
	if c.core != nil {
		defer func() {
			// if err := recover(); err != nil {
			// 	fmt.Fprintf(os.Stderr, "core write data panic error %s", err)
			// }
		}()
		c.core.writeData(data)
	}
}

func (c *BaseCore) addFields(fields []zapcore.Field) {
	for i := range fields {
		c.withfields = append(c.withfields, fields[i])
	}
}

func mergeEntryFields(ent zapcore.Entry, fields []zapcore.Field) {
	fields = append(fields, zapcore.Field{
		Type:   zapcore.StringType,
		Key:    "level",
		String: ent.Level.String(),
	}, zapcore.Field{
		Type:      zapcore.TimeType,
		Key:       "time",
		Integer:   ent.Time.UnixNano(),
		Interface: ent.Time.Location(),
	}, zapcore.Field{
		Type:   zapcore.StringType,
		Key:    "msg",
		String: ent.Message,
	})
}

// CombineFields ..
func CombineFields(src, src2 map[string]string) (dst map[string]string) {
	dst = make(map[string]string)
	for k, v := range src {
		dst[k] = v
	}
	for k, v := range src2 {
		dst[k] = v
	}
	return
}

// ParseLevel .. parse level
func ParseLevel(loglevel string) zapcore.Level {
	var lv zapcore.Level
	lv.UnmarshalText([]byte(loglevel))
	return lv
}

// SliceFieldToMap ..
func SliceFieldToMap(fields []zapcore.Field) map[string]zapcore.Field {
	var fs = make(map[string]zapcore.Field)
	for _, field := range fields {
		fs[field.Key] = field
	}
	return fs
}

// MapFieldToSlice ..
func MapFieldToSlice(fields map[string]zapcore.Field) []zapcore.Field {
	var fs = make([]zapcore.Field, 0, len(fields))
	for _, f := range fields {
		fs = append(fs, f)
	}
	return fs
}

func getfilters(fields []string) (filters map[string]bool) {
	filters = make(map[string]bool)
	for _, f := range fields {
		filters[f] = true
	}
	return
}

// AllLevels Supported log levels
var AllLevels = []zapcore.Level{
	zapcore.DebugLevel,
	zapcore.InfoLevel,
	zapcore.WarnLevel,
	zapcore.ErrorLevel,
	zapcore.FatalLevel,
	zapcore.PanicLevel,
}
