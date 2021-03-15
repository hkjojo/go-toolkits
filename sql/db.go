package sql

import (
	"context"
	"runtime"
	"time"

	goqu "github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	"github.com/pkg/errors"

	"gorm.io/gorm"
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

// Config ..
type Config struct {
	Debug           bool
	Dialect         string
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	TransTimeout    time.Duration
}

// Inject init db conns, panic if fail
// for convenient useage
func Inject(cfg *Config) {
	var err error
	DefaultDB, err = Open(cfg)
	if err != nil {
		panic(err)
	}
}

// Open get opened db instance
func Open(cfg *Config) (*DataBase, error) {
	db, err := NewGorm(cfg.Dialect, cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.Debug {
		db = db.Debug()
	}

	conn, _ := db.DB()
	if cfg.MaxOpenConns != 0 {
		conn.SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns != 0 {
		conn.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime != 0 {
		conn.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	return &DataBase{
		DB:   db,
		cfg:  cfg,
		goqu: goqu.New(cfg.Dialect, conn),
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
		tx.Begin()
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
