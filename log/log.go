package log

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hkjojo/go-toolkits/log/v2/encoder"
	"github.com/hkjojo/go-toolkits/log/v2/hook"
	"github.com/jinzhu/copier"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config ...
type Config struct {
	Path          string                `json:"path"`
	Level         string                `json:"level"`
	Fields        map[string]string     `json:"fields"`
	MaxSize       int                   `json:"max_size"`
	MaxBackups    int                   `json:"max_backups"`
	MaxAge        int                   `json:"max_age"`
	DisableStdout bool                  `json:"disable_stdout"`
	Compress      bool                  `json:"compress"`
	Format        string                `json:"format"` // json/console/text
	ForbidTime    bool                  `json:"forbid_time"`
	ForbidLevel   bool                  `json:"forbid_level"`
	Caller        bool                  `json:"caller"`
	Prefix        string                `json:"prefix"`
	Kafka         *hook.KafkaConfig     `json:"kafka"`
	WebHook       []*hook.WebHookConfig `json:"webhook"`
	RotateDay     int                   `json:"rotate_day"`
}

func (c *Config) Metric() *Config {
	var metricCfg Config
	if c != nil {
		_ = copier.Copy(&metricCfg, c)
	}
	metricCfg.Path = "log/metric.log"
	metricCfg.DisableStdout = true
	return &metricCfg
}

// SugaredLogger ..
type SugaredLogger struct {
	*zap.SugaredLogger
}

// Logger ..
type Logger struct {
	*zap.Logger
	config *Config
}

var (
	// std is the name of the standard logger in stdlib `log`
	logger = &Logger{}
	sugger = &SugaredLogger{}
)

// CoreType ..
type CoreType int

// CoreDefine
const (
	CoreUndefine CoreType = iota
	CoreTelegram
	CoreDingDing
	CoreKafKa
)

func init() {
	l, _ := zap.NewDevelopment()
	logger = &Logger{l, &Config{}}
	sugger = logger.Sugar()
}

// AddFields ..
func (c *Config) AddFields(fs map[string]string) {
	if c.Fields == nil {
		c.Fields = make(map[string]string)
	}
	for k, v := range fs {
		c.Fields[k] = v
	}
}

// Sugar copy zaplog
func (log *Logger) Sugar() *SugaredLogger {
	return &SugaredLogger{log.Logger.Sugar()}
}

// Fields ...
type Fields map[string]interface{}

// New ..
func New(config *Config) (*Logger, error) {
	var (
		lvl        zapcore.Level
		err        error
		hooks      []zapcore.WriteSyncer
		rotatehook *rotatelogs.RotateLogs
		ecoder     zapcore.Encoder
		timeKey    = "time"
		levelKey   = "level"
		msgKey     = "msg"
	)
	if config.Level != "" {
		lvl = hook.ParseLevel(config.Level)
		if err != nil {
			return nil, err
		}
	}

	if config.Path != "" {
		dir := getDir(config.Path)
		if isPathNotExist(dir) {
			if err = os.MkdirAll(dir, os.ModePerm); err != nil {
				return nil, err
			}
		}

		if config.MaxSize != 0 {
			h := lumberjack.Logger{
				Filename:   config.Path,       // log path
				MaxSize:    config.MaxSize,    // file max size：M
				MaxBackups: config.MaxBackups, // max backup file num
				MaxAge:     config.MaxAge,     // file age
				Compress:   config.Compress,   // compress gz
			}
			hooks = append(hooks, zapcore.AddSync(&h))
		}

		if config.RotateDay != 0 {
			var fn = config.Path
			if !filepath.IsAbs(fn) {
				v, err := filepath.Abs(fn)
				if err != nil {
					return nil, err
				}
				fn = v
			}

			rotatehook, err = rotatelogs.New(
				fn+".%Y%m%d",
				rotatelogs.WithLinkName(fn),
				rotatelogs.WithMaxAge(time.Hour*24*time.Duration(config.MaxAge)),
				rotatelogs.WithRotationTime(time.Hour*24*time.Duration(config.RotateDay)),
			)
			if err != nil {
				return nil, err
			}
			hooks = append(hooks, zapcore.AddSync(rotatehook))
		}
	}

	if !config.DisableStdout {
		hooks = append(hooks, os.Stdout)
	}

	if config.ForbidTime {
		timeKey = ""
	}

	if config.ForbidLevel {
		levelKey = ""
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        timeKey,
		LevelKey:       levelKey,
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     msgKey,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	switch strings.ToLower(config.Format) {
	case "json":
		ecoder = zapcore.NewJSONEncoder(encoderConfig)
	case "fix":
		ecoder = encoder.NewFixEncoder(encoderConfig)
	default:
		ecoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	var cores []zapcore.Core
	cores = append(cores, zapcore.NewCore(
		ecoder,
		zapcore.NewMultiWriteSyncer(hooks...),
		lvl,
	))

	for _, cfg := range config.WebHook {
		cores = append(cores, hook.NewWebHookCore(cfg, encoderConfig))
	}

	if config.Kafka != nil {
		core, err := hook.NewKafkaCore(config.Kafka, config.Prefix, config.Fields, encoderConfig)
		if err != nil {
			return nil, err
		}
		cores = append(cores, core)
	}

	core := zapcore.NewTee(cores...)
	var l *zap.Logger
	l = zap.New(core)
	if config.Caller {
		l = l.WithOptions(zap.AddCaller())
	}

	return &Logger{l, config}, nil
}

// Init ...
func Init(config *Config) error {
	var err error
	logger, err = New(config)
	if err != nil {
		return err
	}
	sugger = logger.Sugar()
	return nil
}

func isPathNotExist(path string) bool {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return true
		}
	}
	return false
}

func getDir(path string) string {
	paths := strings.Split(path, "/")
	return strings.Join(
		paths[:len(paths)-1],
		"/",
	)
}

// WithExt ...
func (s *SugaredLogger) WithExt(args ...interface{}) *SugaredLogger {
	return &SugaredLogger{s.SugaredLogger.With(args...)}
}
