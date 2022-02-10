package sql

import (
	"context"
	"runtime"
	"time"

	goqu "github.com/doug-martin/goqu/v9"
	"github.com/pkg/errors"

	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// DefaultDB ...
var DefaultDB *DataBase

// Gorm get gorm db instance
func Gorm() *gorm.DB {
	return DefaultDB.Gorm()
}

// Goqu get gorm db instance
func Goqu() *goqu.Database {
	return DefaultDB.Goqu()
}

// IsNotFound ..
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// WithDialector ...
func WithDialector(dialector gorm.Dialector) gorm.Option {
	return &gorm.Config{
		Dialector: dialector,
	}
}

// Config ..
type Config struct {
	Dialect         Dialect
	URL             string
	TransTimeout    time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Debug           bool
}

// Inject init db conns
// for convenient useage
func Inject(cfg *Config, opts ...gorm.Option) error {
	var err error
	DefaultDB, err = Open(cfg, opts...)
	return err
}

// Open get opened db instance
func Open(cfg *Config, opts ...gorm.Option) (*DataBase, error) {
	opts = append(opts, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})

	var dialector gorm.Dialector
	for _, opt := range opts {
		cfg, ok := opt.(*gorm.Config)
		if ok && cfg.Dialector != nil {
			dialector = cfg.Dialector
		}
	}

	if dialector == nil {
		switch cfg.Dialect {
		case MySQL:
			dialector = mysql.Open(cfg.URL)
		case SQLite3:
			dialector = sqlite.Open(cfg.URL)
		case ClickHouse:
			dialector = clickhouse.Open(cfg.URL)
		case Postgres:
			dialector = postgres.Open(cfg.URL)
		default:
			return nil, ErrUnsupportDriver
		}
	}

	db, err := gorm.Open(dialector, opts...)
	if err != nil {
		return nil, err
	}

	if cfg.Debug {
		db = db.Debug()
	}

	conn, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.MaxOpenConns != 0 {
		conn.SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns != 0 {
		conn.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime != 0 {
		conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	goquDB, err := NewGoqu(cfg.Dialect, conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &DataBase{
		DB:   db,
		cfg:  cfg,
		goqu: goquDB,
	}, nil
}

// DataBase ...
type DataBase struct {
	*gorm.DB
	cfg  *Config
	goqu *goqu.Database
}

// Gorm ...
func (db *DataBase) Gorm() *gorm.DB {
	return db.DB
}

// Goqu ...
func (db *DataBase) Goqu() *goqu.Database {
	return db.goqu
}

// Begin ..
func (db *DataBase) Begin() *gorm.DB {
	return db.DB.Begin()
}

// Commit ..
func (db *DataBase) Commit() *gorm.DB {
	return db.DB.Commit()
}

// Rollback ..
func (db *DataBase) Rollback() *gorm.DB {
	return db.DB.Rollback()
}

// Transaction ...
func (db *DataBase) Transaction(f func(*gorm.DB) error) (err error) {
	return db.TransactionCtx(context.Background(), f)
}

// TransactionCtx ...
func (db *DataBase) TransactionCtx(ctx context.Context, f func(*gorm.DB) error) (err error) {
	var tx *gorm.DB
	if _, ok := ctx.Deadline(); db.cfg.TransTimeout != 0 && !ok {
		ctxt, cancel := context.WithTimeout(ctx, db.cfg.TransTimeout)
		defer cancel()
		tx = db.WithContext(ctxt).Begin()
	} else {
		tx = db.Begin()
	}

	defer func() {
		if ret := recover(); ret != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			err = errors.Errorf("panic[%s] \nret[%v]", string(buf[:n]), ret)
		}
	}()

	err = f(tx)
	if err != nil {
		tx.Rollback()
		return
	}

	err = tx.Commit().Error
	return
}

// Close ...
func (db *DataBase) Close() {
	conn, err := db.Gorm().DB()
	if err == nil {
		conn.Close()
	}
}
