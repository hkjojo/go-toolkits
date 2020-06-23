package microtools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/reader"
	"github.com/micro/go-micro/v2/config/source"
	"github.com/micro/go-micro/v2/config/source/file"
	"github.com/micro/go-plugins/config/source/consul/v2"
)

var cfg = &Config{}

// Option ...
type Option func(*Config)

// Config ...
type Config struct {
	source  source.Source
	from    string
	prefix  string
	address string
	path    []string
	wathers []config.Watcher
}

func (c *Config) init() {
	switch {
	case strings.HasPrefix(c.from, "consul://"):
		// consul path
		c.address = c.from[9:]
		c.path = strings.Split(c.address, "/")
		opts := []source.Option{
			consul.WithAddress(GetRegistryAddress()),
		}

		opts = append(opts, consul.WithPrefix("/"))
		if len(c.path) > 0 {
			opts = append(opts, consul.WithPrefix(c.path[0]))
		}

		c.source = consul.NewSource(opts...)
	case strings.HasPrefix(c.from, "file://"):
		// file path
		c.address = c.from[7:]
		c.source = file.NewSource(
			file.WithPath(c.address),
		)
	default:
		c.address = c.from
		c.source = file.NewSource(
			file.WithPath(c.address),
		)
	}
}

// Get ....
func (c *Config) Get(x interface{}, path ...string) error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	if err = conf.Load(c.source); err != nil {
		return err
	}

	defer conf.Close()

	if err := conf.Get(append(c.path, path...)...).Scan(x); err != nil {
		return err
	}

	return nil
}

// Value ....
func (c *Config) Value(path ...string) (reader.Value, error) {
	conf, err := config.NewConfig()
	if err != nil {
		return nil, err
	}

	if err = conf.Load(c.source); err != nil {
		return nil, err
	}

	defer conf.Close()

	return conf.Get(append(c.path, path...)...), nil
}

// Watch ...
func (c *Config) Watch(scanFunc func(reader.Value, error), path ...string) error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	if err = conf.Load(c.source); err != nil {
		return err
	}
	conf.Sync()

	ps := append(c.path, path...)
	w, err := conf.Watch(ps...)
	if err != nil {
		return err
	}

	c.wathers = append(c.wathers, w)
	go func() {
		val := conf.Get(ps...)
		scanFunc(val, nil)

		for {
			v, err := w.Next()
			if err != nil {
				scanFunc(nil, err)
				return
			}

			scanFunc(v, nil)
		}
	}()

	return nil
}

// Put ...
func (c *Config) Put(config interface{}, path ...string) error {
	if !strings.HasPrefix(c.from, "consul://") {
		return fmt.Errorf("put fail: %s", "source not support")
	}

	data, _ := json.MarshalIndent(config, "", "\t")

	apiConf := api.DefaultConfig()
	apiConf.Address = GetRegistryAddress()

	// Get a new client
	client, err := api.NewClient(apiConf)
	if err != nil {
		return err
	}

	// Get a handle to the KV API
	kv := client.KV()

	// PUT a new KV pair
	key := strings.Join(append(c.path, path...), "/")
	p := &api.KVPair{Key: key, Value: data}
	_, err = kv.Put(p, nil)
	if err != nil {
		return fmt.Errorf("put fail: %w", err)
	}
	return nil
}

// WatchStop ...
func (c *Config) WatchStop() {
	for _, v := range c.wathers {
		v.Stop()
	}
}

// NewConfig ...
func NewConfig(opts ...Option) *Config {
	conf := &Config{from: GetConfigAddress()}
	for _, o := range opts {
		o(conf)
	}
	conf.init()
	return conf
}

// InitSource Directly init source. Use it without micro service
func InitSource(opts ...Option) {
	cfg.from = GetConfigAddress()
	for _, o := range opts {
		o(cfg)
	}
	cfg.init()
}

// WithFrom ...
func WithFrom(from string) Option {
	return func(cfg *Config) {
		cfg.from = from
	}
}

// ConfigGet ...
func ConfigGet(x interface{}, path ...string) error {
	return cfg.Get(x, path...)
}

// ConfigValue ...
func ConfigValue(path ...string) (reader.Value, error) {
	return cfg.Value(path...)
}

// ConfigWatch ...
func ConfigWatch(scanFunc func(reader.Value, error), path ...string) error {
	return cfg.Watch(scanFunc, path...)
}

// ConfigWatchStop ...
func ConfigWatchStop() {
	cfg.WatchStop()
}

// ConfigPut ...
func ConfigPut(config interface{}, path ...string) error {
	return cfg.Put(config, path...)
}
