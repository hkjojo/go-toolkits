package log

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// KafkaConfig ..
type KafkaConfig struct {
	CoreConfig
	Hosts     []string
	Topic     string
	MergeData bool
}

// KafkaCore ..
type KafkaCore struct {
	*BaseCore

	prefix string
	config *KafkaConfig
	client sarama.AsyncProducer
}

// NewKafkaCore ...
func NewKafkaCore(config *Config, encode zapcore.EncoderConfig) (core *KafkaCore, err error) {
	kafka := config.Kafka
	core = &KafkaCore{
		BaseCore: &BaseCore{
			queue:        make(chan *CoreData, kafka.QueueLength),
			LevelEnabler: zap.NewAtomicLevelAt(ParseLevel(kafka.Level)),
			enc:          zapcore.NewJSONEncoder(encode),
			out:          zapcore.AddSync(ioutil.Discard),
			filters:      getfilters(kafka.Filter),
			fields:       CombineFields(config.Fields, kafka.Fields),
			off:          kafka.Off,
		},
		config: kafka,
		prefix: config.Prefix,
	}

	core.BaseCore.core = core
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Partitioner = sarama.NewRandomPartitioner
	cfg.Producer.Return.Successes = true
	cfg.Producer.Timeout = time.Second

	core.client, err = sarama.NewAsyncProducer(kafka.Hosts, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"[log] new kafka client err: %v\n", err)
		return
	}

	go func(p sarama.AsyncProducer) {
		errors := p.Errors()
		success := p.Successes()
		for {
			select {
			case err := <-errors:
				if err != nil {
					fmt.Fprintf(os.Stderr,
						"[log] push kafka fail err: %v\n", err)
				}
			case <-success:
			}
		}
	}(core.client)
	core.start()
	return core, nil
}

func (c *KafkaCore) writeData(data *CoreData) {
	var keys = make(map[string]interface{})
	var str string
	for _, file := range data.fields {
		if !c.config.MergeData {
			keys[file.Key] = c.getField(file)
			continue
		}
		if file.Key == "level" || file.Key == "msg" || file.Key == "time" {
			keys[file.Key] = c.getField(file)
			continue
		}

		if _, ok := c.fields[file.Key]; ok {
			keys[file.Key] = c.getField(file)
			continue
		}

		if str != "" {
			str += fmt.Sprintf(" %s:%v", file.Key, c.getField(file))
		} else {
			str += fmt.Sprintf("%s:%v", file.Key, c.getField(file))
		}
	}

	if str != "" {
		keys["data"] = str
	}

	var content, _ = json.Marshal(keys)
	msg := &sarama.ProducerMessage{}
	msg.Topic = c.prefix + c.config.Topic

	msg.Value = sarama.ByteEncoder(content)
	c.client.Input() <- msg
}

func (c *KafkaCore) close() {
	c.client.Close()
}
