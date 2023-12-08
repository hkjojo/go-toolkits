package redis

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

func TestPubSub(t *testing.T) {
	Init(&Config{MasterAddr: "redis://:BpazRIZL1m@127.0.0.1:6379/2"},
		WithPubSub(),
		WithPubSubReSubCallBack(func() {
			fmt.Printf("resub")
		}))
	var script = `
		local position = {Source = 'dcasia', STDSymbol = 'XAUUSD', LP = 'maxxtrader', LPSymbol = 'XAUUSD', Quantity = 10000, Update = 0}
    	redis.call('PUBLISH', 'position_update', cjson.encode(position))
	`

	err := Subscribe("position_update", func(data string) {
		t.Logf("position_update:%s", data)
	})
	err = Subscribe("position_update_engine", func(data string) {
		t.Logf("position_update_engine:%s", data)
	})
	if err != nil {
		log.Fatal(err)
	}

	s := redis.NewScript(0, script)
	s.Do(defaultPool.Conn())

	time.Sleep(5 * time.Minute)
	// Unsubscribe from the channel
	UnSubscribe("position_update")
	UnSubscribe("position_update_engine")
}
