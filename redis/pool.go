package redis

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gomodule/redigo/redis"
)

// Pool ...
type Pool struct {
	pool           *redis.Pool
	scripts        map[string]*redis.Script
	scriptCallback func(string, string)
	startPubSub    bool
	pubSubClient   *PubSubClient
	reSubCallBack  func()
}

// Option ...
type Option func(*Pool)

func WithLoadScriptCallback(callback func(string, string)) Option {
	return func(p *Pool) {
		p.scriptCallback = callback
	}
}

func WithPubSub() Option {
	return func(p *Pool) {
		p.startPubSub = true
	}
}

func WithPubSubReSubCallBack(cb func()) Option {
	return func(p *Pool) {
		p.reSubCallBack = cb
	}
}

func (p *Pool) loadScript(script string) error {
	return filepath.Walk(script,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			var (
				fileName   = info.Name()
				fileprefix string
				count      string
			)

			if !strings.HasSuffix(fileName, LuaFileSuffix) {
				return nil
			}

			fileprefix = strings.TrimSuffix(fileName, LuaFileSuffix)
			for i := len(fileprefix) - 1; i >= 0; i-- {
				if fileprefix[i] != '_' {
					continue
				}

				count = fileprefix[i+1:]
				c, err := strconv.ParseInt(count, 10, 64)
				if err != nil {
					return nil
				}

				f, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}

				s := redis.NewScript(int(c), string(f))
				conn := p.Conn()
				err = s.Load(conn)
				if err != nil {
					return err
				}
				conn.Close()
				p.scripts[fileprefix] = s
				if p.scriptCallback != nil {
					p.scriptCallback(fileprefix, s.Hash())
				}
				return nil
			}
			return nil
		})
}

// Close ...
func (p *Pool) Close() {
	p.pool.Close()
}

// Get ...
func (p *Pool) Conn() redis.Conn {
	return p.pool.Get()
}

// SendScript ...
func (p *Pool) SendScript(script string, f ReplyFunc, args ...interface{}) error {
	s := p.scripts[script]
	if s == nil {
		return errors.New("not found script")
	}
	var conn = p.Conn()
	defer conn.Close()
	replay, err := s.Do(conn, args...)
	if f != nil {
		return f(replay, err)
	}
	return err
}

// BulkScript ...
func (p *Pool) BulkScript(script string, args [][]interface{}) error {
	s := p.scripts[script]
	if s == nil {
		return errors.New("not found script")
	}
	var conn = p.Conn()
	defer conn.Close()

	for _, arg := range args {
		s.SendHash(conn, arg...)
	}
	conn.Flush()
	_, err := conn.Receive()
	return err
}

// Set ...
func (p *Pool) Set(key, value string) error {
	var (
		conn = p.Conn()
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
func (p *Pool) GetSet(key, value string) (string, error) {
	var (
		conn = p.Conn()
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
func (p *Pool) SetNX(key, value string) (int, error) {
	var (
		conn = p.Conn()
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
func (p *Pool) SetEX(key, value string, seconds int) (int, error) {
	var conn = p.Conn()
	defer conn.Close()

	return redis.Int(conn.Do("SETEX", key, seconds, value))
}

// HSet ...
func (p *Pool) HSet(key string, field, value string) error {
	var (
		conn = p.Conn()
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
func (p *Pool) HIncrBy(key string, field string, value int) (int, error) {
	var (
		conn = p.Conn()
	)

	defer conn.Close()

	return redis.Int(conn.Do("HINCRBY", key, field, value))
}

// HMSet ...
func (p *Pool) HMSet(key string, value interface{}) error {
	var (
		conn = p.Conn()
		err  error
	)

	defer conn.Close()

	_, err = conn.Do("HMSET", redis.Args{}.Add(key).AddFlat(value)...)
	return err
}

// BulkHMSet ...
func (p *Pool) BulkHMSet(values map[string]interface{}) error {
	var (
		conn = p.Conn()
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
func (p *Pool) SAdd(key string, member string) error {
	var (
		conn = p.Conn()
	)

	defer conn.Close()
	_, err := conn.Do("SADD", key, member)
	return err
}

// SRem ...
func (p *Pool) SRem(key string, member string) error {
	var (
		conn = p.Conn()
	)

	defer conn.Close()
	_, err := conn.Do("SREM", key, member)
	return err
}

// Smembers ...
func (p *Pool) Smembers(key string) ([]string, error) {
	var (
		conn = p.Conn()
	)

	defer conn.Close()
	return redis.Strings(conn.Do("SMEMBERS", key))
}

// HGet ...
func (p *Pool) HGet(key string, field string) (string, error) {
	var (
		conn = p.Conn()
	)

	defer conn.Close()

	return redis.String(conn.Do("HGET", key, field))
}

// HGetAll ...
func (p *Pool) HGetAll(key string, value interface{}) error {
	var (
		conn = p.Conn()
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

// HGetAllStringMap ...
func (p *Pool) HGetAllStringMap(key string) (map[string]string, error) {
	var (
		conn = p.Conn()
		err  error
	)

	defer conn.Close()

	v, err := redis.StringMap(conn.Do("HGETALL", key))
	if err != nil {
		return nil, err
	}
	return v, nil
}

// Get ...
func (p *Pool) Get(key string) (string, error) {

	var (
		conn  = p.Conn()
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
func (p *Pool) ScanHGets(key string, f func([]interface{}) error) error {
	var (
		conn   = p.Conn()
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
func (p *Pool) ScanDels(key string) error {
	var (
		conn = p.Conn()
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
func (p *Pool) Dels(key ...interface{}) error {
	var (
		conn = p.Conn()
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
func (p *Pool) Do(command string, args ...interface{}) (interface{}, error) {
	conn := p.Conn()
	defer conn.Close()
	return conn.Do(command, args...)
}
