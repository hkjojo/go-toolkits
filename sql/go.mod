module github.com/hkjojo/go-toolkits/sql

go 1.16

require (
	github.com/DATA-DOG/go-sqlmock v1.4.1 // indirect
	github.com/doug-martin/goqu/v9 v9.11.1
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/hashicorp/go-version v1.3.0 // indirect
	github.com/lib/pq v1.3.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.2.0 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
	gorm.io/driver/clickhouse v0.1.0
	gorm.io/driver/mysql v1.0.5
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.8
)

retract (
	v1.1.2
	v1.1.1
	v1.1.0
)
