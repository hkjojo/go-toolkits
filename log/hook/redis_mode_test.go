package hook

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap/zapcore"
)

func TestRedisStreamClientOptions(t *testing.T) {
	cases := []struct {
		name        string
		cfg         RedisStreamConfig
		wantAddrs   []string
		wantCluster bool
		wantErr     string
	}{
		{name: "default is single", cfg: RedisStreamConfig{Addr: "redis:6379"}, wantAddrs: []string{"redis:6379"}},
		{name: "explicit single keeps db", cfg: RedisStreamConfig{Mode: "single", Addr: "redis:6379", DB: 2}, wantAddrs: []string{"redis:6379"}},
		{name: "single rejects a node list", cfg: RedisStreamConfig{Addr: "a:6379,b:6379"}, wantErr: "mode: cluster"},
		{name: "cluster from one seed", cfg: RedisStreamConfig{Mode: "cluster", Addr: "cfg.endpoint:6379"}, wantAddrs: []string{"cfg.endpoint:6379"}, wantCluster: true},
		{name: "cluster seed list is trimmed", cfg: RedisStreamConfig{Mode: " Cluster ", Addr: "a:6379, b:6379,"}, wantAddrs: []string{"a:6379", "b:6379"}, wantCluster: true},
		{name: "cluster rejects non-zero db", cfg: RedisStreamConfig{Mode: "cluster", Addr: "a:6379", DB: 1}, wantErr: "db must be 0"},
		{name: "cluster rejects empty seeds", cfg: RedisStreamConfig{Mode: "cluster", Addr: " , "}, wantErr: "addr is required"},
		{name: "unknown mode", cfg: RedisStreamConfig{Mode: "sentinel", Addr: "a:6379"}, wantErr: "unknown mode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts, err := redisStreamClientOptions(&c.cfg)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(opts.Addrs, c.wantAddrs) {
				t.Errorf("addrs = %q, want %q", opts.Addrs, c.wantAddrs)
			}
			if opts.IsClusterMode != c.wantCluster {
				t.Errorf("IsClusterMode = %v, want %v", opts.IsClusterMode, c.wantCluster)
			}
		})
	}
}

// go-redis builds a single-node client from a single address unless IsClusterMode is set;
// a single-node client on a cluster gets MOVED for every key on another node. The client
// type is what actually decides routing, so assert it rather than the option flag alone.
func TestRedisStreamClientOptions_ClientType(t *testing.T) {
	for _, c := range []struct {
		mode string
		want any
	}{
		{mode: "", want: &redis.Client{}},
		{mode: RedisModeCluster, want: &redis.ClusterClient{}},
	} {
		opts, err := redisStreamClientOptions(&RedisStreamConfig{Mode: c.mode, Addr: "127.0.0.1:1"})
		if err != nil {
			t.Fatalf("mode %q: %v", c.mode, err)
		}
		cl := redis.NewUniversalClient(opts)
		if got, want := reflect.TypeOf(cl), reflect.TypeOf(c.want); got != want {
			t.Errorf("mode %q: client type = %v, want %v", c.mode, got, want)
		}
		_ = cl.Close()
	}
}

func TestRedisStreamConstructors_RejectBadMode(t *testing.T) {
	bad := func() *RedisStreamConfig { return &RedisStreamConfig{Mode: "cluster", Addr: "a:6379", DB: 3} }

	if _, err := NewRedisStreamCore(bad(), "", nil, zapcore.EncoderConfig{}); err == nil || !strings.Contains(err.Error(), "db must be 0") {
		t.Errorf("core: err = %v, want db must be 0", err)
	}
	saver := func(context.Context, []Entry) error { return nil }
	if _, err := NewRedisStreamConsumer(bad(), ConsumerOpts{GroupName: "g", ConsumerID: "c"}, saver); err == nil || !strings.Contains(err.Error(), "db must be 0") {
		t.Errorf("consumer: err = %v, want db must be 0", err)
	}
}
