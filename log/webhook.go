package log

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Method
const (
	MethodGET  = "GET"
	MethodPOST = "POST"
)

// WebHookConfig ..
type WebHookConfig struct {
	CoreConfig
	Host        string
	Message     string
	KVMessage   string
	Method      string
	ContentType string
}

// WebHookCore ..
type WebHookCore struct {
	*BaseCore

	config *WebHookConfig
}

// NewWebHookCore ...
func NewWebHookCore(config *WebHookConfig, encode zapcore.EncoderConfig) (core *WebHookCore) {
	core = &WebHookCore{
		BaseCore: &BaseCore{
			queue:        make(chan *CoreData, config.QueueLength),
			LevelEnabler: zap.NewAtomicLevelAt(ParseLevel(config.Level)),
			enc:          zapcore.NewJSONEncoder(encode),
			out:          zapcore.AddSync(ioutil.Discard),
			filters:      getfilters(config.Filter),
			fields:       CombineFields(config.Fields, config.Fields),
			off:          config.Off,
		},
		config: config,
	}
	core.BaseCore.core = core
	core.start()
	return
}

func (c *WebHookCore) writeData(data *CoreData) {
	var (
		req     = c.config.Message
		content string
		rsp     *http.Response
		err     error
	)

	for _, f := range data.fields {
		var kv = strings.Replace(c.config.KVMessage, "{{key}}", f.Key, -1)
		kv = strings.Replace(kv, "{{value}}", c.getFieldString(f), -1)
		content += kv
	}

	req = strings.Replace(req, "{{content}}", content, -1)

	switch c.config.Method {
	case MethodGET:
		//rsp, err = http.Get(config.Host + req)
	case MethodPOST:
		rsp, err = http.Post(c.config.Host, c.config.ContentType,
			bytes.NewBuffer([]byte(req)))
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "web hook fail host:%s err:%v content:%s rsp:%v\n",
			c.config.Host, err, req, rsp)
	}
	if rsp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "web hook fail host:%s code:%d content:%s rsp:%v\n",
			c.config.Host, rsp.StatusCode, req, rsp)
	}
	fmt.Println(req)
}
