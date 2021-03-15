package sql

import (
	"database/sql"
	"errors"

	"github.com/doug-martin/goqu/v9"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var (
	ErrUnsupportDriver = errors.New("unsupport db driver")
)

func NewGorm(dialect, url string) (*gorm.DB, error) {
	var (
		dialector gorm.Dialector
	)
	switch dialect {
	case "mysql":
		dialector = mysql.Open(url)
	case "sqlite":
		dialector = sqlite.Open(url)
	case "clickhouse":
		dialector = clickhouse.Open(url)
	default:
		return nil, ErrUnsupportDriver
	}
	return gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
}

func NewGoqu(dialect string, conn *sql.DB) *goqu.Database {
	switch dialect {
	case "clickhouse":
		return goqu.New("mysql", conn)
	default:
		return goqu.New(dialect, conn)
	}
}
