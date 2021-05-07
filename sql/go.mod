module github.com/hkjojo/go-toolkits/sql

go 1.16

require (
	github.com/ClickHouse/clickhouse-go v1.4.5 // indirect
	github.com/doug-martin/goqu/v9 v9.12.0
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.2.0 // indirect
	gorm.io/driver/clickhouse v0.1.0
	gorm.io/driver/mysql v1.0.6
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.9
)

retract (
	v1.1.2
	v1.1.1
	v1.1.0
)
