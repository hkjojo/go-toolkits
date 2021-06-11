package redis

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FZambia/sentinel"
	"github.com/gomodule/redigo/redis"
)

var (
	DefaultMaxActive      = 50
	DefaultMaxIdle        = 100
	DefaultIdleTimeout    = 2 * time.Minute
	DefaultConnectTimeout = 5 * time.Second
	DefaultReadTimeout    = 5 * time.Second
	DefaultWriteTimeout   = 5 * time.Second
	LuaFileSuffix         = ".lua"

	rdsPool *Pool
)

type Config struct {
	MasterAddr string
	Script     string
	Sentinels  []string
	ReadOnly   bool
}

type Script struct {
	Src      string
	KeyCount int
}

type ReplyFunc func(interface{}, error) error

func Close() {
	rdsPool.Close()
}

func Default() *Pool {
	return rdsPool
}

func Init(conf *Config, loadScript func(string, string)) error {
	switch {
	case len(conf.Sentinels) != 0:
		rdsPool = NewSentinel(conf.Sentinels, conf.ReadOnly)
	case len(conf.MasterAddr) != 0:
		rdsPool = New(conf.MasterAddr)
	default:
		return errors.New("redis address empty")
	}
	if err := rdsPool.Get().Err(); err != nil {
		return err
	}
	return rdsPool.loadScript(conf.Script, loadScript)
}

// New ...
func New(url string) *Pool {
	if url == "" {
		url = "redis://127.0.0.1:6379"
	} else {
		if !strings.HasPrefix(url, "redis://") {
			url = "redis://" + url
		}
	}
	return &Pool{
		pool: &redis.Pool{
			MaxIdle:     DefaultMaxIdle,
			MaxActive:   DefaultMaxActive,
			IdleTimeout: DefaultIdleTimeout,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				c, err := redis.DialURL(url,
					redis.DialConnectTimeout(DefaultConnectTimeout),
					redis.DialReadTimeout(DefaultReadTimeout),
					redis.DialWriteTimeout(DefaultWriteTimeout))
				if err != nil {
					return nil, err
				}
				return c, nil
			},
		},
		scripts: make(map[string]*redis.Script),
	}
}

// NewSentinel ...
func NewSentinel(url []string, readOnly bool) *Pool {
	if len(url) == 0 {
		url = []string{"redis://tasks.sentinel:26379"}
	}

	for k, v := range url {
		if !strings.HasPrefix(v, "redis://") {
			url[k] = "redis://" + v
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
			MaxIdle:     DefaultMaxIdle,
			MaxActive:   DefaultMaxActive,
			IdleTimeout: DefaultIdleTimeout,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				var (
					slaves   []string
					redisURL string
					err      error
				)

				if readOnly {
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

				return redis.DialURL("redis://"+redisURL,
					redis.DialConnectTimeout(DefaultConnectTimeout),
					redis.DialReadTimeout(DefaultReadTimeout),
					redis.DialWriteTimeout(DefaultWriteTimeout),
				)
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				if time.Since(t) < time.Minute {
					return nil
				}

				if readOnly {
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
	s := rdsPool.scripts[script]
	if s == nil {
		return errors.New("not found script")
	}
	var conn = rdsPool.Get()
	defer conn.Close()
	replay, err := s.Do(conn, args...)
	if f != nil {
		return f(replay, err)
	}
	return err
}

// BulkScript ...
func BulkScript(script string, args [][]interface{}) error {
	s := rdsPool.scripts[script]
	if s == nil {
		return errors.New("not found script")
	}
	var conn = rdsPool.Get()
	defer conn.Close()

	for _, arg := range args {
		s.SendHash(conn, arg...)
	}
	conn.Flush()
	_, err := conn.Receive()
	return err
}

// Set ...
func Set(key, value string) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	_, err = conn.Do("SET", key, value)
	if err != nil {
		return err
	}

	return nil
}

// GetSet ...
func GetSet(key, value string) (string, error) {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	value, err = redis.String(conn.Do("GETSET", key, value))
	if err != nil {
		return "", err
	}

	return value, nil

}

// SetNX ...
func SetNX(key, value string) (int, error) {
	var (
		conn = rdsPool.Get()
		err  error
		ret  int
	)

	defer conn.Close()

	ret, err = redis.Int(conn.Do("SETNX", key, value))
	if err != nil {
		return ret, err
	}

	return ret, nil
}

// SetEX ...
func SetEX(key, value string, seconds int) (int, error) {
	var conn = rdsPool.Get()
	defer conn.Close()

	return redis.Int(conn.Do("SETEX", key, seconds, value))
}

// HSet ...
func HSet(key string, field, value string) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	_, err = conn.Do("HSET", key, field, value)
	if err != nil {
		return err
	}

	return nil
}

// HIncrBy ...
func HIncrBy(key string, field string, value int) (int, error) {
	var (
		conn = rdsPool.Get()
	)

	defer conn.Close()

	return redis.Int(conn.Do("HINCRBY", key, field, value))
}

// HMSet ...
func HMSet(key string, value interface{}) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	_, err = conn.Do("HMSET", redis.Args{}.Add(key).AddFlat(value)...)
	return err
}

// BulkHMSet ...
func BulkHMSet(values map[string]interface{}) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	for key, value := range values {
		conn.Send("HMSET", redis.Args{}.Add(key).AddFlat(value)...)
	}
	_, err = redis.Values(conn.Do("EXEC"))
	return err
}

// SAdd ...
func SAdd(key string, member string) error {
	var (
		conn = rdsPool.Get()
	)

	defer conn.Close()
	_, err := conn.Do("SADD", key, member)
	return err
}

// SRem ...
func SRem(key string, member string) error {
	var (
		conn = rdsPool.Get()
	)

	defer conn.Close()
	_, err := conn.Do("SREM", key, member)
	return err
}

// Smembers ...
func Smembers(key string) ([]string, error) {
	var (
		conn = rdsPool.Get()
	)

	defer conn.Close()
	return redis.Strings(conn.Do("SMEMBERS", key))
}

// HGet ...
func HGet(key string, field string) (string, error) {
	var (
		conn = rdsPool.Get()
	)

	defer conn.Close()

	return redis.String(conn.Do("HGET", key, field))
}

// HGetAll ...
func HGetAll(key string, value interface{}) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	v, err := redis.Values(conn.Do("HGETALL", key))
	if err != nil {
		return err
	}

	if err := redis.ScanStruct(v, value); err != nil {
		return err
	}

	return nil
}

// Get ...
func Get(key string) (string, error) {

	var (
		conn  = rdsPool.Get()
		err   error
		value string
	)

	defer conn.Close()

	value, err = redis.String(conn.Do("GET", key))
	if err != nil && err == redis.ErrNil {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	return value, nil
}

// ScanHGets ...
// Note: Use SCAN instead of KEYS, KEYS will block the server
func ScanHGets(key string, f func([]interface{}) error) error {
	var (
		conn   = rdsPool.Get()
		err    error
		values []interface{}
		keys   []string
	)

	defer conn.Close()

	iter := 0
	for {
		arr, err := redis.Values(conn.Do("SCAN", iter, "MATCH", key))
		if err != nil {
			return err
		}

		iter, _ = redis.Int(arr[0], nil)
		k, _ := redis.Strings(arr[1], nil)
		keys = append(keys, k...)

		if iter == 0 {
			break
		}
	}
	conn.Send("MULTI")
	for _, key := range keys {
		conn.Send("HGETALL", key)
	}
	values, err = redis.Values(conn.Do("EXEC"))
	if err != nil && err == redis.ErrNil {
		return nil
	}

	if err != nil {
		return err
	}

	if f != nil {
		return f(values)
	}
	return nil

}

// ScanDels ...
// Note: Use SCAN instead of KEYS, KEYS will block the server
func ScanDels(key string) error {
	var (
		conn = rdsPool.Get()
		err  error
		keys []string
	)

	defer conn.Close()

	iter := 0
	for {
		arr, err := redis.Values(conn.Do("SCAN", iter, "MATCH", key))
		if err != nil {
			return err
		}

		iter, _ = redis.Int(arr[0], nil)
		k, _ := redis.Strings(arr[1], nil)
		keys = append(keys, k...)

		if iter == 0 {
			break
		}
	}
	conn.Send("MULTI")
	for i := range keys {
		conn.Send("DEL", keys[i])
	}
	_, err = redis.Values(conn.Do("EXEC"))
	if err != nil && err == redis.ErrNil {
		return nil
	}
	return err
}

// Dels ...
func Dels(key ...interface{}) error {
	var (
		conn = rdsPool.Get()
		err  error
	)

	defer conn.Close()

	_, err = redis.Int(conn.Do("DEL", key...))
	if err != nil {
		return err
	}
	return nil
}

// Do ...
func Do(command string, args ...interface{}) (interface{}, error) {
	conn := rdsPool.Get()
	defer conn.Close()
	return conn.Do(command, args...)
}

// UnderlyingPool ...
func UnderlyingPool() *redis.Pool {
	return rdsPool.pool
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
//  Reply type    Result
//  float         reply, nil
//  bulk string   parsed reply, nil
//  nil           0, ErrNil
//  other         0, error
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
