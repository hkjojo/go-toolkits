package main

import (
	"time"
	"unsafe"

	"github.com/go-kratos/kratos/v2/log"
	tlog "github.com/hkjojo/go-toolkits/log/v2"
	tlogk "github.com/hkjojo/go-toolkits/log/v2/kratos"
	"github.com/hkjojo/go-toolkits/metric"
	"github.com/prometheus/client_golang/prometheus"
)

import "C"

var clearup func()

var (
	counters        = make(map[string]metric.Counter)
	observers       = make(map[string]metric.Observer)
	gauges          = make(map[string]metric.Gauge)
	mt5State        metric.Gauge
	mt5GatewayState metric.Gauge
)

const (
	MetaKey_HOSTNAME = "hostname"
	MetaKey_SERVICE  = "service"
	MetaKey_VERSION  = "version"
	MetaKey_ENV      = "env"
	MetaKey_TAG      = "tag"
)

func main() {

}

//export Start
func Start(
	serverName *C.char,
	version *C.char,
	hostname *C.char,
	env *C.char,
	tag *C.char,
	logPath *C.char,
	mtStateEnable bool,
	mtGatewayStateEnable bool,
	outErr **C.char,
) {
	logger, err := tlogk.NewZapLog(&tlog.Config{
		Path:      C.GoString(logPath),
		Level:     "debug",
		MaxAge:    30,
		RotateDay: 1,
		Format:    "json",
	})
	if err != nil {
		*outErr = C.CString(err.Error())
		return
	}

	clearup, err = metric.Start(
		metric.WithInterval(time.Second*10),
		metric.WithJSONLoggerWriter(
			tlogk.NewHelper(
				log.With(logger,
					MetaKey_ENV, C.GoString(env),
					MetaKey_TAG, C.GoString(tag),
					MetaKey_HOSTNAME, C.GoString(hostname),
					MetaKey_SERVICE, C.GoString(serverName),
					MetaKey_VERSION, C.GoString(version),
				),
			),
		),
	)
	if err != nil {
		*outErr = C.CString(err.Error())
		return
	}

	if mtStateEnable {
		mt5State = metric.NewGauge(metric.MT5StateGauge)
	}
	if mtGatewayStateEnable {
		mt5GatewayState = metric.NewGauge(metric.MT5GatewayStateGauge)
	}
}

//export Close
func Close() {
	if clearup != nil {
		clearup()
	}
}

//export MT5StateSet
func MT5StateSet(value float64, lvs **C.char, lvsLen C.int) {
	if mt5State == nil {
		return
	}

	var gauge = mt5State
	if lvsLen > 0 {
		gauge = mt5State.With(parse2Strings(lvs, lvsLen)...)
	}

	gauge.Set(value)
}

//export MT5GatewayStateSet
func MT5GatewayStateSet(value float64, lvs **C.char, lvsLen C.int) {
	if mt5GatewayState == nil {
		return
	}

	var gauge = mt5GatewayState
	if lvsLen > 0 {
		gauge = mt5GatewayState.With(parse2Strings(lvs, lvsLen)...)
	}

	gauge.Set(value)
}

//export MT5GatewayStateDelete
func MT5GatewayStateDelete(lvs **C.char, lvsLen C.int) {
	if mt5GatewayState == nil {
		return
	}

	if lvsLen > 0 {
		mt5GatewayState.Delete(parse2Strings(lvs, lvsLen)...)
	}
}

//export AddCounter
func AddCounter(nameSpace *C.char, subsystem *C.char, name *C.char, help *C.char,
	labelNames **C.char, labelNamesLen C.int) {
	counters[C.GoString(name)] = metric.NewCounter(prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: C.GoString(nameSpace),
			Subsystem: C.GoString(subsystem),
			Name:      C.GoString(name),
			Help:      C.GoString(help),
		}, parse2Strings(labelNames, labelNamesLen)))
}

//export AddGauge
func AddGauge(nameSpace *C.char, subsystem *C.char, name *C.char, help *C.char,
	labelNames **C.char, labelNamesLen C.int) {
	gauges[C.GoString(name)] = metric.NewGauge(prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: C.GoString(nameSpace),
			Subsystem: C.GoString(subsystem),
			Name:      C.GoString(name),
			Help:      C.GoString(help),
		}, parse2Strings(labelNames, labelNamesLen)))
}

//export AddHistogram
func AddHistogram(nameSpace *C.char, subsystem *C.char, name *C.char, help *C.char,
	buckets []float64, labelNames **C.char, labelNamesLen C.int) {
	observers[C.GoString(name)] = metric.NewHistogram(prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: C.GoString(nameSpace),
			Subsystem: C.GoString(subsystem),
			Name:      C.GoString(name),
			Help:      C.GoString(help),
			Buckets:   buckets,
		}, parse2Strings(labelNames, labelNamesLen)))
}

//export CounterAdd
func CounterAdd(name *C.char, delta float64, lvs **C.char, lvsLen C.int) {
	counter := counters[C.GoString(name)]
	if counter == nil {
		return
	}
	if lvsLen > 0 {
		counter = counter.With(parse2Strings(lvs, lvsLen)...)
	}

	counter.Add(delta)
}

//export QuoteAdd
func QuoteAdd(symbol *C.char, delta float64) {
	counter := counters["quote_count"]
	if counter == nil {
		return

	}

	counter.With(C.GoString(symbol)).Add(delta)
}

//export GaugeSet
func GaugeSet(name *C.char, value float64, lvs **C.char, lvsLen C.int) {
	gauge := gauges[C.GoString(name)]
	if gauge == nil {
		return

	}
	if lvsLen > 0 {
		gauge = gauge.With(parse2Strings(lvs, lvsLen)...)
	}

	gauge.Set(value)
}

//export GaugeAdd
func GaugeAdd(name *C.char, delta float64, lvs **C.char, lvsLen C.int) {
	gauge := gauges[C.GoString(name)]
	if gauge == nil {
		return

	}

	if lvsLen > 0 {
		gauge = gauge.With(parse2Strings(lvs, lvsLen)...)
	}

	gauge.Add(delta)
}

//export HistogramObserver
func HistogramObserver(name *C.char, value float64, lvs **C.char, lvsLen C.int) {
	observer := observers[C.GoString(name)]
	if observer == nil {
		return

	}

	if lvsLen > 0 {
		observer = observer.With(parse2Strings(lvs, lvsLen)...)
	}

	observer.Observe(value)
}

func parse2Strings(argv **C.char, argc C.int) []string {
	length := int(argc)
	tmpslice := unsafe.Slice(argv, length)
	gostrings := make([]string, length)
	for i, s := range tmpslice {
		gostrings[i] = C.GoString(s)
	}

	return gostrings
}
