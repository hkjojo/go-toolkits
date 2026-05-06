# Log

## Telegrame

- Telegram search BotFather,type cmd(/newbot) to create token and robot
- Add robot by robot_name to group
- Send a dummy message to the bot /my_id @my_bot
- https://api.telegram.org/botXXXXX/getUpdates, XXX as token
- get the chat_id
- use https://api.telegram.org/botXXXXX/sendMessage as URL
- char_id as CharID

## DingDing

- Create DingDing group
- Add Group Assistant(Custom robot)
- Get Webhook url as URL

## Redis Stream Hook

Producer-side core that XADDs each log entry into a Redis stream. Designed
for cross-service runtime event aggregation (e.g., a system_log table where
N services produce and a single consumer persists). The hook builds its own
`redis.UniversalClient` from `RedisStreamConfig` — no external client
required.

```go
import (
    tlog "github.com/hkjojo/go-toolkits/log/v2"
    "github.com/hkjojo/go-toolkits/log/v2/hook"
)

cfg := &tlog.Config{
    Level:  "info",
    Format: "json",
    Caller: true,
    Fields: map[string]string{"service": "maker-adapter"},
    Redis: &hook.RedisStreamConfig{
        Addr:      "redis:6379",
        StreamKey: "system_logs",
        MaxLen:    200_000,
        QPS:       50,
        // Filter is the BaseCore drop list — listed field keys are removed
        // before serialization (e.g., to keep secrets out of logs).
        CoreConfig: hook.CoreConfig{
            Filter: []string{"password", "api_key", "secret"},
        },
    },
}
logger, _ := tlog.New(cfg)
```

Each XADD writes the following fields:

| Field     | Source                                             |
|-----------|----------------------------------------------------|
| `ts`      | unix milliseconds (string)                         |
| `level`   | zap raw level (`debug`/`info`/`warn`/`error`/...)  |
| `service` | `RedisStreamConfig.Fields["service"]`              |
| `source`  | sub-logger field via `log.With("source", "...")`   |
| `msg`     | zap entry message                                  |
| `payload` | remaining structured fields, JSON-encoded          |
| `host`    | `os.Hostname()` (auto-injected)                    |

Consumer side (parses the same wire format and feeds a user-provided
`SaverFunc` for batch persistence):

```go
consumer, _ := hook.NewRedisStreamConsumer(cfg.Redis, hook.ConsumerOpts{
    GroupName:  "system-log-persister",
    ConsumerID: "apiserver-" + os.Getenv("HOSTNAME"),
}, func(ctx context.Context, batch []hook.Entry) error {
    return repo.BulkInsertSystemLog(ctx, batch)
})
_ = consumer.Start(ctx)
defer consumer.Stop()
```

The consumer runs three internal goroutines:

1. main loop — `XReadGroup` + saver + `XAck`
2. autoclaim loop — periodic `XAutoClaim` of stale PEL entries
3. cleanup loop — periodic `XGroupDelConsumer` for idle consumers

Failure modes:

- redis ping fail at startup → stderr warning, core continues, sink degrades silently
- saver returns error → batch left in PEL, reclaimed on next autoclaim cycle
- producer storm → token-bucket throttle (`QPS`) drops over-budget entries; observable via `(*RedisStreamCore).DroppedTotal()`
- oversized payload → replaced with `{"_truncated":true}` (cap via `MaxPayloadBytes`)
