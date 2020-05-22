package microtools

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/micro/go-micro/v2/config"
	"github.com/micro/go-micro/v2/config/reader"
	"github.com/micro/go-micro/v2/config/source"
	"github.com/micro/go-plugins/config/source/consul/v2"
)

type conf struct {
	source  source.Source
	prefix  string
	address string
}

var consulConf = &conf{}

func getPrefixedPath(path ...string) []string {
	prefixPaths := strings.Split(consulConf.prefix, "/")
	path = append(prefixPaths[1:], path...)

	return path
}

// GetAddress ..
func GetAddress() string {
	return consulConf.address
}

// InitSource Directly init source. Use it without micro service
func InitSource(address string, prefix string) {
	consulConf.address = address
	var opts = []source.Option{consul.WithAddress(consulConf.address)}
	consulConf.prefix = prefix
	opts = append(opts,
		consul.WithPrefix(consulConf.prefix),
		consul.StripPrefix(true),
	)
	consulConf.source = consul.NewSource(opts...)
}

// ConfigGet ...
func ConfigGet(x interface{}, path ...string) error {
	conf, err := config.NewConfig()
	if err != nil {
		return err
	}

	if err = conf.Load(consulConf.source); err != nil {
		return err
	}

	defer conf.Close()

	if err := conf.Get(path...).Scan(x); err != nil {
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

	if err = conf.Load(consulConf.source); err != nil {
		return err
	}

	w, err := conf.Watch(path...)
	if err != nil {
		return err
	}

	go func() {
		val := conf.Get(path...)
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
func Sync(service string, conf interface{}) error {
	data, _ := json.MarshalIndent(conf, "", "\t")

	apiConf := api.DefaultConfig()
	apiConf.Address = consulConf.address

	// Get a new client
	client, err := api.NewClient(apiConf)
	if err != nil {
		return err
	}

	// Get a handle to the KV API
	kv := client.KV()

	// PUT a new KV pair
	p := &api.KVPair{Key: service, Value: data}
	_, err = kv.Put(p, nil)
	return err
}
