package redis

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

type PositionState struct {
	Source    string  `redis:"src" json:"src"`
	STDSymbol string  `redis:"stdsym" json:"stdsym"`
	LP        string  `redis:"lp" json:"lp"`
	LPSymbol  string  `redis:"lpsym" json:"lpsym"`
	Quantity  float64 `redis:"qty" json:"qty"`
}

func TestPubSub(t *testing.T) {
	Init(&Config{MasterAddr: "redis://:BpazRIZL1m@127.0.0.1:6379/2"},
		WithPubSub(),
		WithPubSubReSubCallBack(func() {
			fmt.Printf("resub")
		}))
	//	var script = `
	//local ps = {}
	//local stdKeys = redis.call('SMEMBERS', 'stp_position_state')
	//for k, stdKey in pairs(stdKeys) do
	//	local keys = redis.call('SMEMBERS', stdKey)
	//	for k, key in pairs(keys) do
	//	    local position = redis.call('HGETALL', key)
	//        if #position ~=0 then
	//            local data = {}
	//            for i = 1, #position, 2 do
	//                data[position[i]] = position[i + 1]
	//            end
	//        	table.insert(ps, data)
	//        end
	//	end
	//end
	//redis.call('PUBLISH', 'position_full_update_engine', cjson.encode(ps))
	//	`

	var script = `
local position = {src = ARGV[1], stdsym = ARGV[2], lp = ARGV[3], lpsym = ARGV[4], qty = tonumber(ARGV[5])}
redis.call('PUBLISH', 'position_update1', cjson.encode(position))
`

	err := Subscribe("position_update1", func(data string) {
		t.Logf("position_update:%s", data)
	})
	err = Subscribe("position_full_update_engine", func(data string) {
		t.Log(data)
		var positions []*PositionState
		//ps, _ := redis.Values(data, nil)
		//for _, p := range ps {
		//	var position PositionState
		//	pos, _ := redis.Values(p, nil)
		//	redis.ScanStruct(pos, &position)
		//	positions = append(positions, &position)
		//}
		json.Unmarshal([]byte(data), &positions)
		for _, position := range positions {
			t.Log(position)
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	s := redis.NewScript(0, script)
	s.Do(defaultPool.Conn(),
		redis.Args{}.
			Add("sail").
			Add("sail").
			Add("sail").
			Add("sail").
			Add(10000.00)...)

	time.Sleep(5 * time.Minute)
	// Unsubscribe from the channel
	UnSubscribe("position_update")
	UnSubscribe("position_update_engine")
}
