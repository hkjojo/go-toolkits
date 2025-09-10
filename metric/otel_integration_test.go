package metric

import (
	"log"
	"os"
	"time"
)

// TestOTELIntegration 测试OTEL集成
func TestOTELIntegration() {
	// 设置环境变量（可选）
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	os.Setenv("OTEL_SERVICE_NAME", "go-toolkits-metric-test")
	os.Setenv("OTEL_SERVICE_VERSION", "1.0.0")
	os.Setenv("ENV", "test")

	// 创建测试日志器
	logger := &testLogger{}

	// 启动metric采集，连接本地OTEL采集器
	stop, err := Start(
		logger,
		WithMode(ModeOTEL),
		WithEndpoint("localhost:4317"),
		WithServiceName("go-toolkits-metric-test"),
		WithInterval(time.Second*10), // 每10秒采集一次
		WithStatsMetric(),            // 启用运行时统计
	)
	if err != nil {
		log.Fatalf("Failed to start metric collection: %v", err)
	}
	defer stop()

	log.Println("Metric collection started successfully!")
	log.Println("Sending metrics to OTEL collector at localhost:4317...")

	// 创建各种类型的指标
	counter := NewCounter("test_requests_total", "Total number of test requests", "1")
	gauge := NewGauge("test_active_connections", "Number of active test connections", "1")
	histogram := NewHistogram("test_request_duration", "Test request duration", "s", 0.1, 0.5, 1.0, 2.0, 5.0)
	summary := NewSummary("test_response_size", "Test response size", "bytes")

	// 模拟业务操作，发送各种指标
	for i := 0; i < 100; i++ {
		// 计数器操作
		counter.With("method", "GET", "status", "200").Inc()
		counter.With("method", "POST", "status", "201").Add(2)
		counter.With("method", "GET", "status", "404").Inc()

		// 仪表盘操作
		gauge.With("service", "api", "instance", "server-1").Set(float64(10 + i%20))
		gauge.With("service", "api", "instance", "server-2").Set(float64(15 + i%15))
		gauge.With("service", "db", "instance", "db-1").Set(float64(5 + i%10))

		// 直方图操作
		duration := float64(i%100) / 10.0 // 0.0 到 9.9 秒
		histogram.With("endpoint", "/api/users", "method", "GET").Observe(duration)
		histogram.With("endpoint", "/api/orders", "method", "POST").Observe(duration + 0.5)

		// 摘要操作
		responseSize := float64(1000 + i%5000) // 1KB 到 6KB
		summary.With("endpoint", "/api/users", "method", "GET").Observe(responseSize)
		summary.With("endpoint", "/api/orders", "method", "POST").Observe(responseSize * 1.5)

		// 每10次操作打印一次日志
		if i%10 == 0 {
			log.Printf("Sent %d batches of metrics...", i+1)
		}

		// 等待1秒再发送下一批
		time.Sleep(time.Second)
	}

	log.Println("Test completed! Check your OTEL collector and backend for metrics.")
	log.Println("You can also check Jaeger UI at http://localhost:16686 if you're using Jaeger as backend.")
}

// TestOTELWithCustomMetrics 测试自定义业务指标
func TestOTELWithCustomMetrics() {
	logger := &testLogger{}

	stop, err := Start(
		logger,
		WithMode(ModeOTEL),
		WithEndpoint("localhost:4317"),
		WithServiceName("business-metrics-test"),
		WithInterval(time.Second*5),
		WithStatsMetric(),
	)
	if err != nil {
		log.Fatalf("Failed to start metric collection: %v", err)
	}
	defer stop()

	log.Println("Testing business metrics with OTEL...")

	// 业务指标
	userRegistrations := NewCounter("user_registrations_total", "Total user registrations", "1")
	activeUsers := NewGauge("active_users", "Number of active users", "1")
	loginDuration := NewHistogram("login_duration", "User login duration", "s", 0.1, 0.5, 1.0, 2.0, 5.0)
	apiLatency := NewSummary("api_latency", "API response latency", "ms")

	// 模拟用户注册
	for i := 0; i < 50; i++ {
		userRegistrations.With("source", "web", "country", "US").Inc()
		userRegistrations.With("source", "mobile", "country", "CN").Inc()
		userRegistrations.With("source", "web", "country", "EU").Add(2)
	}

	// 模拟活跃用户数变化
	for i := 0; i < 30; i++ {
		activeUsers.With("region", "us-east").Set(float64(1000 + i*10))
		activeUsers.With("region", "us-west").Set(float64(800 + i*8))
		activeUsers.With("region", "eu-west").Set(float64(1200 + i*12))
	}

	// 模拟登录时长
	for i := 0; i < 100; i++ {
		duration := float64(i%50) / 10.0 // 0.0 到 4.9 秒
		loginDuration.With("method", "password", "success", "true").Observe(duration)
		loginDuration.With("method", "oauth", "success", "true").Observe(duration * 0.8)
		loginDuration.With("method", "password", "success", "false").Observe(duration * 1.5)
	}

	// 模拟API延迟
	for i := 0; i < 200; i++ {
		latency := float64(10 + i%100) // 10ms 到 109ms
		apiLatency.With("endpoint", "/api/v1/users", "method", "GET").Observe(latency)
		apiLatency.With("endpoint", "/api/v1/orders", "method", "POST").Observe(latency * 1.2)
		apiLatency.With("endpoint", "/api/v1/products", "method", "GET").Observe(latency * 0.9)
	}

	log.Println("Business metrics test completed!")
}

// TestOTELKratosMiddleware 测试Kratos中间件集成
func TestOTELKratosMiddleware() {
	logger := &testLogger{}

	stop, err := Start(
		logger,
		WithMode(ModeOTEL),
		WithEndpoint("localhost:4317"),
		WithServiceName("kratos-middleware-test"),
		WithInterval(time.Second*5),
		WithStatsMetric(),
	)
	if err != nil {
		log.Fatalf("Failed to start metric collection: %v", err)
	}
	defer stop()

	log.Println("Testing Kratos middleware with OTEL...")

	// 创建Kratos中间件使用的指标
	requestsCounter := NewCounter("server_requests_total", "Total number of server requests", "1")
	requestDuration := NewHistogram("server_request_duration", "Server request duration", "s", 0.01, 0.1, 0.5, 1.0, 2.0, 5.0)

	// 模拟HTTP请求
	endpoints := []string{"/api/v1/users", "/api/v1/orders", "/api/v1/products", "/api/v1/payments"}
	methods := []string{"GET", "POST", "PUT", "DELETE"}
	statusCodes := []string{"200", "201", "400", "404", "500"}

	for i := 0; i < 100; i++ {
		endpoint := endpoints[i%len(endpoints)]
		method := methods[i%len(methods)]
		statusCode := statusCodes[i%len(statusCodes)]

		// 记录请求计数
		requestsCounter.With("method", method, "endpoint", endpoint, "status", statusCode).Inc()

		// 记录请求时长
		duration := float64(i%100) / 100.0 // 0.0 到 0.99 秒
		requestDuration.With("method", method, "endpoint", endpoint).Observe(duration)

		if i%20 == 0 {
			log.Printf("Simulated %d HTTP requests...", i+1)
		}
	}

	log.Println("Kratos middleware test completed!")
}

// testLogger 实现Logger接口用于测试
type testLogger struct{}

func (t *testLogger) Errorw(msg string, args ...interface{}) {
	log.Printf("[ERROR] %s %v", msg, args)
}

func (t *testLogger) Warnw(msg string, args ...interface{}) {
	log.Printf("[WARN] %s %v", msg, args)
}

func (t *testLogger) Infow(msg string, args ...interface{}) {
	log.Printf("[INFO] %s %v", msg, args)
}

// RunAllTests 运行所有测试
func RunAllTests() {
	log.Println("Starting OTEL integration tests...")
	log.Println("Make sure your OTEL collector is running at localhost:4317")
	log.Println("")

	// 测试1: 基本OTEL集成
	log.Println("=== Test 1: Basic OTEL Integration ===")
	TestOTELIntegration()
	log.Println("")

	// 等待一下
	time.Sleep(time.Second * 2)

	// 测试2: 自定义业务指标
	log.Println("=== Test 2: Custom Business Metrics ===")
	TestOTELWithCustomMetrics()
	log.Println("")

	// 等待一下
	time.Sleep(time.Second * 2)

	// 测试3: Kratos中间件集成
	log.Println("=== Test 3: Kratos Middleware Integration ===")
	TestOTELKratosMiddleware()
	log.Println("")

	log.Println("All tests completed!")
	log.Println("Check your OTEL collector logs and backend for received metrics.")
}
