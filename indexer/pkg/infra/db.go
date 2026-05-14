package infra

import (
	"net/url"
	"strings"
	"time"

	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/constant"
	"github.com/fystack/multichain-indexer/b2b-platform/pkg/common/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewDBConnection(dsn string, environment string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	host := "unknown"
	dbname := "unknown"
	if u, err := url.Parse(dsn); err == nil {
		host = u.Host
		dbname = strings.TrimPrefix(u.Path, "/")
	}

	logger.Info("Database connection established!", "host", host, "database", dbname)

	if environment != constant.EnvProduction {
		// only print debug logs when not in production
		db = db.Debug()
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool. sqlDB.SetMaxIdleConns(10)

	// axOpenConns sets the maximum number of open connections to the database.
	sqlDB.SetMaxOpenConns(100)

	// onnMaxLifetime sets the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}
