dll:
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -buildmode=c-shared -o Metric.dll metric.go

cdll:
	gcc -shared -pthread -o Metric.dll Metric.c Metric.go.dll -lWinMM -lntdll -lWS2_32