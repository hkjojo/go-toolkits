package sql

import (
	"database/sql"
	"errors"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
)

var (
	ErrUnsupportDriver = errors.New("unsupport db driver")
)

type Dialect string

const (
	MySQL      Dialect = "mysql"
	SQLite3    Dialect = "sqlite3"
	ClickHouse Dialect = "clickhouse"
	Postgres   Dialect = "postgres"
)

func NewGoqu(dialect Dialect, conn *sql.DB) (*goqu.Database, error) {
	switch dialect {
	case ClickHouse, MySQL:
		return goqu.New("mysql", conn), nil
	case SQLite3:
		return goqu.New("sqlite3", conn), nil
	case Postgres:
		return goqu.New("postgres", conn), nil
	default:
		return nil, ErrUnsupportDriver
	}
}
