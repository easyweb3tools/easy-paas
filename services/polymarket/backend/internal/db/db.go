package db

import (
	"database/sql"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"polymarket/internal/config"
)

type DB struct {
	Gorm *gorm.DB
	SQL  *sql.DB
}

func Open(cfg config.DBConfig) (*DB, error) {
	gcfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	gdb, err := gorm.Open(postgres.Open(cfg.DSN), gcfg)
	if err != nil {
		return nil, err
	}

	sqldb, err := gdb.DB()
	if err != nil {
		return nil, err
	}

	sqldb.SetMaxOpenConns(cfg.MaxOpenConns)
	sqldb.SetMaxIdleConns(cfg.MaxIdleConns)
	sqldb.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqldb.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return &DB{Gorm: gdb, SQL: sqldb}, nil
}

func Close(db *DB) error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

func Ping(db *DB) error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Ping()
}

func SetTimezone(db *DB, tz string) error {
	if tz == "" {
		return nil
	}
	_, err := db.SQL.Exec("SET TIME ZONE '" + tz + "'")
	return err
}

func NowUTC() time.Time {
	return time.Now().UTC()
}
