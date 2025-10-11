package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hkjojo/go-toolkits/metric"
)

func main() {
	var (
		testType = flag.String("test", "all", "Test type: all, basic, business, kratos")
		duration = flag.Duration("duration", 0, "Test duration (0 means run once)")
	)
	flag.Parse()

	log.Println("OTEL Metric Integration Test")
	log.Println("===========================")
	log.Println("Make sure your OTEL collector is running at localhost:4317")
	log.Println("You can start OTEL collector with:")
	log.Println("  docker run -p 4317:4317 -p 4318:4318 otel/opentelemetry-collector-contrib:latest")
	log.Println("")

	// 设置环境变量
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	os.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	os.Setenv("OTEL_SERVICE_NAME", "go-toolkits-metric-demo")
	os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
	os.Setenv("ENV", "demo")

	// 创建日志器
	logger := &demoLogger{}

	// 启动metric采集
	stop, err := metric.Start(
		logger,
		metric.WithMode(metric.ModeOTEL),
		metric.WithInterval(time.Second*5),
		metric.WithStatsMetric(),
	)
	if err != nil {
		log.Fatalf("Failed to start metric collection: %v", err)
	}
	defer stop()

	log.Println("Metric collection started successfully!")

	// 根据测试类型运行不同的测试
	switch *testType {
	case "basic":
		runBasicTest()
	case "business":
		runBusinessTest()
	case "kratos":
		runKratosTest()
	case "all":
		runAllTests()
	default:
		log.Fatalf("Unknown test type: %s", *testType)
	}

	// 如果设置了持续时间，则持续运行
	if *duration > 0 {
		log.Printf("Running continuously for %v...", *duration)

		// 设置信号处理
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// 持续发送指标
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					sendContinuousMetrics()
				case <-sigChan:
					log.Println("Received interrupt signal, stopping...")
					return
				}
			}
		}()

		// 等待指定时间或信号
		select {
		case <-time.After(*duration):
			log.Println("Test duration completed")
		case <-sigChan:
			log.Println("Received interrupt signal, stopping...")
		}
	}

	log.Println("Test completed!")
}

func runBasicTest() {
	log.Println("=== Running Basic Test ===")

	counter := metric.NewCounter("", "", "demo_requests_total", "Total demo requests", []string{"1"})
	gauge := metric.NewGauge("", "", "demo_active_connections", "Active demo connections", []string{"1"})
	histogram := metric.NewHistogram("", "", "demo_request_duration", "Demo request duration", []string{"s"}, 0.1, 0.5, 1.0, 2.0, 5.0)

	for i := 0; i < 50; i++ {
		counter.With("method", "GET", "status", "200").Inc()
		gauge.With("service", "demo").Set(float64(10 + i))
		histogram.With("endpoint", "/demo").Observe(float64(i%100) / 10.0)
		time.Sleep(time.Millisecond * 100)
	}
}

func runBusinessTest() {
	log.Println("=== Running Business Test ===")

	userRegistrations := metric.NewCounter("", "", "user_registrations_total", "Total user registrations", []string{"1"})
	activeUsers := metric.NewGauge("", "", "active_users", "Number of active users", []string{"1"})
	loginDuration := metric.NewHistogram("", "", "login_duration", "User login duration", []string{"s"}, 0.1, 0.5, 1.0, 2.0, 5.0)

	for i := 0; i < 30; i++ {
		userRegistrations.With("source", "web", "country", "US").Inc()
		activeUsers.With("region", "us-east").Set(float64(1000 + i*10))
		loginDuration.With("method", "password").Observe(float64(i%50) / 10.0)
		time.Sleep(time.Millisecond * 200)
	}
}

func runKratosTest() {
	log.Println("=== Running Kratos Test ===")

	requestsCounter := metric.NewCounter("", "", "server_requests_total", "Total server requests", []string{"1"})
	requestDuration := metric.NewHistogram("", "", "server_request_duration", "Server request duration", []string{"s"}, 0.01, 0.1, 0.5, 1.0, 2.0)

	endpoints := []string{"/api/v1/users", "/api/v1/orders", "/api/v1/products"}
	methods := []string{"GET", "POST", "PUT"}
	statusCodes := []string{"200", "201", "400", "404"}

	for i := 0; i < 40; i++ {
		endpoint := endpoints[i%len(endpoints)]
		method := methods[i%len(methods)]
		statusCode := statusCodes[i%len(statusCodes)]

		requestsCounter.With("method", method, "endpoint", endpoint, "status", statusCode).Inc()
		requestDuration.With("method", method, "endpoint", endpoint).Observe(float64(i%100) / 100.0)
		time.Sleep(time.Millisecond * 150)
	}
}

func runAllTests() {
	log.Println("=== Running All Tests ===")
	runBasicTest()
	time.Sleep(time.Second)
	runBusinessTest()
	time.Sleep(time.Second)
	runKratosTest()
}

func sendContinuousMetrics() {
	counter := metric.NewCounter("", "", "continuous_requests_total", "Continuous requests", []string{"1"})
	gauge := metric.NewGauge("", "", "continuous_active_connections", "Continuous active connections", []string{"1"})

	counter.With("service", "continuous").Inc()
	gauge.With("service", "continuous").Set(float64(time.Now().Unix() % 1000))
}

// demoLogger 实现Logger接口
type demoLogger struct{}

func (d *demoLogger) Errorw(msg string, args ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, args)
}

func (d *demoLogger) Warnw(msg string, args ...interface{}) {
	log.Printf("[WARN] %s %v", msg, args)
}

func (d *demoLogger) Infow(msg string, args ...interface{}) {
	log.Printf("[INFO] %s %v", msg, args)
}
