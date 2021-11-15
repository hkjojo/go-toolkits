module github.com/hkjojo/go-toolkits/sql

go 1.16

require (
	github.com/ClickHouse/clickhouse-go v1.5.1 // indirect
	github.com/doug-martin/goqu/v9 v9.18.0
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.2.0 // indirect
	golang.org/x/crypto v0.0.0-20211108221036-ceb1ce70b4fa // indirect
	gorm.io/driver/clickhouse v0.2.1
	gorm.io/driver/mysql v1.2.0
	gorm.io/driver/postgres v1.2.2
	gorm.io/driver/sqlite v1.2.4
	gorm.io/gorm v1.22.3
)

retract (
	v1.1.2
	v1.1.1
	v1.1.0
)
