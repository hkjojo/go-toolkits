package sql

import (
	"database/sql"
	"errors"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	ErrUnsupportDriver = errors.New("unsupport db driver")
)

type Dialect string

const (
	MySQL      Dialect = "mysql"
	SQLite3    Dialect = "sqlite3"
	ClickHouse Dialect = "clickhouse"
)

func NewGorm(dialect Dialect, url string, opts ...gorm.Option) (*gorm.DB, error) {
	var (
		dialector gorm.Dialector
	)

	switch dialect {
	case MySQL:
		dialector = mysql.Open(url)
	case SQLite3:
		dialector = sqlite.Open(url)
	case ClickHouse:
		dialector = clickhouse.Open(url)
	default:
		return nil, ErrUnsupportDriver
	}

	cfgs := make([]gorm.Option, 0, len(opts)+1)
	cfgs = append(cfgs, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	cfgs = append(cfgs, opts...)
	return gorm.Open(dialector, cfgs...)
}

func NewGoqu(dialect Dialect, conn *sql.DB) (*goqu.Database, error) {
	switch dialect {
	case ClickHouse, MySQL:
		return goqu.New("mysql", conn), nil
	case SQLite3:
		return goqu.New("sqlite3", conn), nil
	default:
		return nil, ErrUnsupportDriver
	}
}
