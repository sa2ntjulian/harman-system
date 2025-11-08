package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/jeongdaeha/harman-k8s-collector/internal/config"
	"github.com/jeongdaeha/harman-k8s-collector/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := cfg.DSN()
	gormLogger := logger.Default.LogMode(logger.Info)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: false,
		PrepareStmt:            true,
	})
	if err != nil {
		return nil, fmt.Errorf("데이터베이스 연결 실패: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("DB 인스턴스 가져오기 실패: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	slog.Info("데이터베이스 연결 성공", "host", cfg.Host, "database", cfg.Database)

	return db, nil
}

func MigrateDB(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.FileCollection{},
		&models.CollectedFile{},
	)
	if err != nil {
		return fmt.Errorf("마이그레이션 실패: %w", err)
	}
	return nil
}

func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("DB 인스턴스 가져오기 실패: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("데이터베이스 연결 종료 실패: %w", err)
	}

	return nil
}

