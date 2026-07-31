package dao

import (
	"hospital-qc-wework/internal/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/denisenkom/go-mssqldb" // SQL Server 驱动
)

// InitDB 初始化数据库连接池
func InitDB(cfg config.DBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlserver", cfg.DSN())
	if err != nil {
		return nil, err
	}

	// 配置连接池
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// 验证连接
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// InitHISDB 初始化 HIS 数据库连接（只读）
func InitHISDB(cfg config.HISDBConfig) (*sqlx.DB, error) {
	db, err := sqlx.Connect("sqlserver", cfg.HISDSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
