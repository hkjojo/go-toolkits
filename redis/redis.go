package redis

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FZambia/sentinel"
	"github.com/gomodule/redigo/redis"
)

var (
	defaultConfig = &Config{
		MaxActive:      50,
		MaxIdle:        100,
		IdleTimeout:    2 * 60,
		ConnectTimeout: 5,
		ReadTimeout:    5,
		WriteTimeout:   5,
	}

	LuaFileSuffix = ".lua"

	defaultPool *Pool
)

type Config struct {
	MasterAddr     string
	Script         string
	Sentinels      []string
	ReadOnly       bool
	Debug          bool
	MaxActive      int
	MaxIdle        int
	IdleTimeout    int
	ConnectTimeout int
	ReadTimeout    int
	WriteTimeout   int
	TLSSkipVerify  bool
}

func (c *Config) merge(conf *Config) {
	if c.MaxActive <= 0 {
		c.MaxActive = conf.MaxActive
	}
	if c.MaxIdle <= 0 {
		c.MaxIdle = conf.MaxIdle
	}
	if c.IdleTimeout <= 0 {
		c.IdleTimeout = conf.IdleTimeout
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = conf.ConnectTimeout
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = conf.ReadTimeout
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = conf.WriteTimeout
	}
}

type Script struct {
	Src      string
	KeyCount int
}

type ReplyFunc func(interface{}, error) error

func Close() {
	defaultPool.Close()
}

func Default() *Pool {
	return defaultPool
}

func Init(conf *Config, opts ...Option) error {
	conf.merge(defaultConfig)
	switch {
	case len(conf.Sentinels) != 0:
		defaultPool = NewSentinel(conf)
	case len(conf.MasterAddr) != 0:
		defaultPool = New(conf)
	default:
		return errors.New("redis address empty")
	}

	for _, opt := range opts {
		opt(defaultPool)
	}

	if err := defaultPool.Conn().Err(); err != nil {
		return err
	}

	if conf.Script != "" {
		err := defaultPool.loadScript(conf.Script)
		if err != nil {
			return err
		}
	}

	if defaultPool.startPubSub {
		client, err := NewPubSubClient(defaultPool, defaultPool.reSubCallBack)
		if err != nil {
			return err
		}
		defaultPool.pubSubClient = client
	}

	return nil
}

func getRedisURL(addr string, tls bool) string {
	var (
		redisURL = addr
		scheme   = "redis://"
	)
	if tls {
		scheme = "rediss://"
	}

	switch {
	case redisURL == "":
		redisURL = fmt.Sprintf("%s%s", scheme, "127.0.0.1:6379")
	case strings.HasPrefix(redisURL, "redis://") && tls:
		redisURL = strings.Replace(redisURL, "redis://", scheme, 1)
	case strings.HasPrefix(redisURL, "rediss://") && !tls:
		redisURL = strings.Replace(redisURL, "rediss://", scheme, 1)
	case !strings.HasPrefix(redisURL, scheme):
		redisURL = fmt.Sprintf("%s%s", scheme, redisURL)
	}
	return redisURL
}

// New ...
func New(conf *Config) *Pool {
	var (
		useTls bool
		url    = conf.MasterAddr
	)

	if url == "" {
		url = "redis://127.0.0.1:6379"
	} else {
		if !strings.HasPrefix(url, "redis://") {
			url = "redis://" + url
		}
	}
	if strings.HasPrefix(url, "rediss://") {
		useTls = true
	}

	var options = []redis.DialOption{
		redis.DialConnectTimeout(time.Duration(conf.ConnectTimeout) * time.Second),
		redis.DialReadTimeout(time.Duration(conf.ReadTimeout) * time.Second),
		redis.DialWriteTimeout(time.Duration(conf.WriteTimeout) * time.Second),
	}
	if useTls {
		options = append(options, redis.DialUseTLS(true))
		options = append(options, redis.DialTLSSkipVerify(conf.TLSSkipVerify))
	}

	return &Pool{
		pool: &redis.Pool{
			MaxIdle:     conf.MaxIdle,
			MaxActive:   conf.MaxActive,
			IdleTimeout: time.Duration(conf.IdleTimeout) * time.Second,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				conn, err := redis.DialURL(url, options...)
				if err != nil {
					return nil, err
				}
				if conf.Debug {
					return redis.NewLoggingConn(conn, log.Default(), ""), nil
				}
				return conn, nil
			},
		},
		scripts: make(map[string]*redis.Script),
	}
}

// NewSentinel ...
func NewSentinel(conf *Config) *Pool {
	var (
		useTls bool
		url    = conf.Sentinels
		scheme = "redis://"
	)

	if len(url) == 0 {
		url = []string{"redis://tasks.sentinel:26379"}
	}

	for k, v := range url {
		if !strings.HasPrefix(v, "redis://") {
			url[k] = "redis://" + v
		}
		if strings.HasPrefix(v, "rediss://") {
			useTls = true
			scheme = "rediss://"
		}
	}
	sentinelCli := sentinel.Sentinel{
		Addrs:      url,
		MasterName: "mymaster",
		Dial: func(addr string) (redis.Conn, error) {
			return redis.DialURL(addr)
		},
	}
	return &Pool{
		pool: &redis.Pool{
			MaxIdle:     conf.MaxIdle,
			MaxActive:   conf.MaxActive,
			IdleTimeout: time.Duration(conf.IdleTimeout) * time.Second,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				var (
					slaves   []string
					redisURL string
					err      error
				)

				if conf.ReadOnly {
					slaves, err = sentinelCli.SlaveAddrs()
					if err != nil {
						return nil, err
					}
				}

				if len(slaves) > 0 {
					redisURL = slaves[0]
				}

				if len(redisURL) == 0 {
					redisURL, err = sentinelCli.MasterAddr()
					if err != nil {
						return nil, err
					}
				}

				var options = []redis.DialOption{
					redis.DialConnectTimeout(time.Duration(conf.ConnectTimeout) * time.Second),
					redis.DialReadTimeout(time.Duration(conf.ReadTimeout) * time.Second),
					redis.DialWriteTimeout(time.Duration(conf.WriteTimeout) * time.Second),
				}
				if useTls {
					options = append(options, redis.DialUseTLS(true))
					options = append(options, redis.DialTLSSkipVerify(conf.TLSSkipVerify))
				}

				conn, err := redis.DialURL(scheme+redisURL, options...)

				if err != nil {
					return nil, err
				}

				if conf.Debug {
					return redis.NewLoggingConn(conn, log.Default(), ""), nil
				}
				return conn, nil
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if time.Since(t) < time.Minute {
					return nil
				}

				if conf.ReadOnly {
					return nil
				}

				if !sentinel.TestRole(c, "master") {
					return errors.New("redis role check failed")
				}
				return nil
			},
		},
		scripts: make(map[string]*redis.Script),
	}
}

// SendScript ...
func SendScript(script string, f ReplyFunc, args ...interface{}) error {
	return defaultPool.SendScript(script, f, args...)
}

// BulkScript ...
func BulkScript(script string, args [][]interface{}) error {
	return defaultPool.BulkScript(script, args)
}

// Set ...
func Set(key, value string) error {
	return defaultPool.Set(key, value)
}

// GetSet ...
func GetSet(key, value string) (string, error) {
	return defaultPool.GetSet(key, value)

}

// SetNX ...
func SetNX(key, value string) (int, error) {
	return defaultPool.SetNX(key, value)
}

// SetEX ...
func SetEX(key, value string, seconds int) (int, error) {
	return defaultPool.SetEX(key, value, seconds)
}

// HSet ...
func HSet(key string, field, value string) error {
	return defaultPool.HSet(key, field, value)
}

// HIncrBy ...
func HIncrBy(key string, field string, value int) (int, error) {
	return defaultPool.HIncrBy(key, field, value)
}

// HMSet ...
func HMSet(key string, value interface{}) error {
	return defaultPool.HMSet(key, value)
}

// BulkHMSet ...
func BulkHMSet(values map[string]interface{}) error {
	return defaultPool.BulkHMSet(values)
}

// SAdd ...
func SAdd(key string, member string) error {
	return defaultPool.SAdd(key, member)
}

// SRem ...
func SRem(key string, member string) error {
	return defaultPool.SRem(key, member)
}

// Smembers ...
func Smembers(key string) ([]string, error) {
	return defaultPool.Smembers(key)
}

// HGet ...
func HGet(key string, field string) (string, error) {
	return defaultPool.HGet(key, field)
}

// HGetAll ...
func HGetAll(key string, value interface{}) error {
	return defaultPool.HGetAll(key, value)
}

// Get ...
func Get(key string) (string, error) {
	return defaultPool.Get(key)
}

// ScanHGets ...
// Note: Use SCAN instead of KEYS, KEYS will block the server
func ScanHGets(key string, f func([]interface{}) error) error {
	return defaultPool.ScanHGets(key, f)

}

// ScanDels ...
// Note: Use SCAN instead of KEYS, KEYS will block the server
func ScanDels(key string) error {
	return defaultPool.ScanDels(key)
}

// Dels ...
func Dels(key ...interface{}) error {
	return defaultPool.Dels(key...)
}

// Do ...
func Do(command string, args ...interface{}) (interface{}, error) {
	return defaultPool.Do(command, args...)
}

// UnderlyingPool ...
func UnderlyingPool() *redis.Pool {
	return defaultPool.pool
}

// ParseURL ...
func ParseURL(redisURL string) (string, []redis.DialOption, error) {
	var (
		options []redis.DialOption
		addr    string
		db      int
	)
	u, err := url.Parse(redisURL)
	if err != nil {
		return addr, nil, err
	}

	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return addr, nil, errors.New("invalid redis URL scheme: " + u.Scheme)
	}

	if u.User != nil {
		if p, ok := u.User.Password(); ok {
			options = append(options, redis.DialPassword(p))
		}
	}

	if len(u.Query()) > 0 {
		return addr, nil, errors.New("no options supported")
	}

	h, p, err := net.SplitHostPort(u.Host)
	if err != nil {
		h = u.Host
	}
	if h == "" {
		h = "localhost"
	}
	if p == "" {
		p = "6379"
	}

	addr = net.JoinHostPort(h, p)

	f := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})

	switch len(f) {
	case 0:
		db = 0
	case 1:
		if db, err = strconv.Atoi(f[0]); err != nil {
			return addr, nil, fmt.Errorf("invalid redis database number: %q", f[0])
		}
	default:
		return addr, nil, errors.New("invalid redis URL path: " + u.Path)
	}

	options = append(options, redis.DialDatabase(db))

	if u.Scheme == "rediss" {
		options = append(options, redis.DialTLSConfig(&tls.Config{ServerName: h}))
	}
	return addr, options, nil
}

// Float64Map is a helper that converts an array of strings (alternating key, value)
// into a map[string]float64. The HGETALL commands return replies in this format.
// Requires an even number of values in result.
func Float64Map(result interface{}, err error) (map[string]float64, error) {
	values, err := redis.Values(result, err)
	if err != nil {
		return nil, err
	}
	if len(values)%2 != 0 {
		return nil, errors.New("redigo: Float64Map expects even number of values result")
	}
	m := make(map[string]float64, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].([]byte)
		if !ok {
			return nil, errors.New("redigo: Float64Map key not a bulk string value")
		}
		value, err := Float64(values[i+1], nil)
		if err != nil {
			return nil, err
		}
		m[string(key)] = value
	}
	return m, nil
}

// Float64 is a helper that converts a command reply to 64 bit float. If err is
// not equal to nil, then Int returns 0, err. Otherwise, Float64 converts the
// reply to an float64 as follows:
//
//	Reply type    Result
//	float         reply, nil
//	bulk string   parsed reply, nil
//	nil           0, ErrNil
//	other         0, error
func Float64(reply interface{}, err error) (float64, error) {
	if err != nil {
		return 0, err
	}
	switch reply := reply.(type) {
	case float64:
		return reply, nil
	case []byte:
		n, err := strconv.ParseFloat(string(reply), 64)
		return n, err
	case nil:
		return 0, redis.ErrNil
	case redis.Error:
		return 0, reply
	}
	return 0, fmt.Errorf("redigo: unexpected type for Int64, got type %T", reply)
}

func Subscribe(channel string, cb CallBack) error {
	if !defaultPool.startPubSub {
		return fmt.Errorf("pubsub not start")
	}

	return defaultPool.pubSubClient.subscribe(channel, cb)
}

func UnSubscribe(channel string) {
	if defaultPool.startPubSub {
		defaultPool.pubSubClient.unsubscribe(channel)
	}

}
