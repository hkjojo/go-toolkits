package hook

import (
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
	CoreConfig `json:",inline"`
	Hosts      []string `json:"hosts"`
	Topic      string   `json:"topic"`
	MergeData  bool     `json:"is_merge_data"`
}

// KafkaCore ..
type KafkaCore struct {
	*BaseCore

	prefix string
	config *KafkaConfig
	client sarama.AsyncProducer
}

// NewKafkaCore ...
func NewKafkaCore(config *KafkaConfig, prefix string, fields map[string]string, encode zapcore.EncoderConfig) (core *KafkaCore, err error) {
	core = &KafkaCore{
		BaseCore: &BaseCore{
			queue:        make(chan *CoreData, config.QueueLength),
			LevelEnabler: zap.NewAtomicLevelAt(ParseLevel(config.Level)),
			enc:          zapcore.NewJSONEncoder(encode),
			out:          zapcore.AddSync(ioutil.Discard),
			filters:      getfilters(config.Filter),
			fields:       CombineFields(fields, config.Fields),
			off:          config.Off,
		},
		config: config,
		prefix: prefix,
	}

	core.BaseCore.core = core
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Partitioner = sarama.NewRandomPartitioner
	cfg.Producer.Return.Successes = true
	cfg.Producer.Timeout = time.Second

	core.client, err = sarama.NewAsyncProducer(config.Hosts, cfg)
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

func (c *KafkaCore) encode(data *CoreData) (string, error) {
	encode := c.enc.Clone()
	var str string
	for _, field := range data.fields {
		if !c.config.MergeData {
			field.AddTo(encode)
			continue
		}

		if _, ok := c.fields[field.Key]; ok {
			field.AddTo(encode)
			continue
		}

		if str != "" {
			str += fmt.Sprintf(" %s:%v", field.Key, c.getField(field))
		} else {
			str += fmt.Sprintf("%s:%v", field.Key, c.getField(field))
		}
	}

	if str != "" {
		zapcore.Field{Key: "data", String: str, Type: zapcore.StringType}.AddTo(encode)
	}

	buf, err := encode.EncodeEntry(data.entry, nil)
	if err != nil {
		return "", err
	}
	defer buf.Free()
	return buf.String(), nil
}

func (c *KafkaCore) writeData(data *CoreData) {
	content, err := c.encode(data)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"[log] kafka encode err: %v\n", err)
		return
	}
	c.write(content)
}

func (c *KafkaCore) write(content string) {
	msg := &sarama.ProducerMessage{}
	msg.Topic = c.prefix + c.config.Topic
	msg.Value = sarama.ByteEncoder(content)
	c.client.Input() <- msg
}

func (c *KafkaCore) close() {
	c.client.Close()
}
