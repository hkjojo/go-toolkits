module github.com/hkjojo/go-toolkits/sql

go 1.16

require (
	github.com/doug-martin/goqu/v9 v9.18.0
	github.com/lib/pq v1.10.2 // indirect
	github.com/mattn/go-sqlite3 v1.14.9 // indirect
	github.com/pkg/errors v0.9.1
	github.com/stretchr/objx v0.2.0 // indirect
	gorm.io/gorm v1.22.3
)

retract (
	v1.1.2
	v1.1.1
	v1.1.0
)
