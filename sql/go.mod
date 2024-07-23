module github.com/hkjojo/go-toolkits/sql

go 1.16

require (
	github.com/doug-martin/goqu/v9 v9.18.0
	github.com/pkg/errors v0.9.1
	gorm.io/driver/clickhouse v0.2.2
	gorm.io/driver/mysql v1.2.3
	gorm.io/driver/postgres v1.2.3
	gorm.io/driver/sqlite v1.5.6
	gorm.io/gorm v1.25.7-0.20240204074919-46816ad31dde
)

retract (
	v1.1.2
	v1.1.1
	v1.1.0
)
