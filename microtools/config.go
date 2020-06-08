package microtools

import (
	"strings"

	"github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/reader"
	"github.com/micro/go-micro/v2/config/source"
	"github.com/micro/go-micro/v2/config/source/file"
	"github.com/micro/go-plugins/config/source/consul/v2"
)

// Option ...
type Option func(*conf)
type conf struct {
	source  source.Source
	from    string
	prefix  string
	address string
	path    []string
}

var cfg = &conf{}

// InitSource Directly init source. Use it without micro service
func InitSource(opts ...Option) {
	cfg.from = GetConfigAddress()
	for _, o := range opts {
		o(cfg)
	}

	switch {
	case strings.HasPrefix(cfg.from, "consul://"):
		// consul path
		cfg.address = cfg.from[9:]
		cfg.path = strings.Split(cfg.address, "/")
		opts := []source.Option{
			consul.WithAddress(GetRegistryAddress()),
		}
		if len(cfg.path) > 0 {
			opts = append(opts, consul.WithPrefix(cfg.path[0]))
		}

		cfg.source = consul.NewSource(opts...)
	case strings.HasPrefix(cfg.from, "file://"):
		// file path
		cfg.address = cfg.from[7:]
		cfg.source = file.NewSource(
			file.WithPath(cfg.address),
		)
	}
}

// WithFrom ...
func WithFrom(from string) Option {
	return func(cfg *conf) {
		cfg.from = from
	}
}

// ConfigGet ...
func ConfigGet(x interface{}, path ...string) error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	if err = conf.Load(cfg.source); err != nil {
		return err
	}

	defer conf.Close()

	if err := conf.Get(append(cfg.path, path...)...).Scan(x); err != nil {
		return err
	}

	return nil
}

// ConfigWatch ...
func ConfigWatch(scanFunc func(reader.Value, error), path ...string) error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	if err = conf.Load(cfg.source); err != nil {
		return err
	}

	ps := append(cfg.path, path...)
	w, err := conf.Watch(ps...)
	if err != nil {
		return err
	}

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

// Sync ...
// func Sync(service string, conf interface{}) error {
// 	data, _ := json.MarshalIndent(conf, "", "\t")

// 	apiConf := api.DefaultConfig()
// 	apiConf.Address = cfg.address

// 	// Get a new client
// 	client, err := api.NewClient(apiConf)
// 	if err != nil {
// 		return err
// 	}

// 	// Get a handle to the KV API
// 	kv := client.KV()

// 	// PUT a new KV pair
// 	p := &api.KVPair{Key: service, Value: data}
// 	_, err = kv.Put(p, nil)
// 	return err
// }
