package sql

import (
	"time"

	goqu "github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/doug-martin/goqu/v9/dialect/sqlite3"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"

	"github.com/jinzhu/gorm"
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
	db, err := gorm.Open(cfg.Dialect, cfg.URL)
	if err != nil {
		return nil, err
	}

	db.SingularTable(true)
	if cfg.Debug {
		db.LogMode(true)
	}

	if cfg.MaxOpenConns != 0 {
		db.DB().SetMaxOpenConns(cfg.MaxOpenConns)
	}

	if cfg.MaxIdleConns != 0 {
		db.DB().SetMaxIdleConns(cfg.MaxIdleConns)
	}

	if cfg.ConnMaxLifetime != 0 {
		db.DB().SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	return &DataBase{
		DB:   db,
		goqu: goqu.New(cfg.Dialect, db.DB()),
	}, nil
}

// DataBase ...
type DataBase struct {
	*gorm.DB
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
func (db *DataBase) Transaction(f func(*gorm.DB) error) error {
	tx := db.Begin()
	err := f(tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		return err
	}

	return nil
}
