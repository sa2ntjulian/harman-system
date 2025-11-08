package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database  DatabaseConfig
	Collector CollectorConfig
	NodeName  string
	LogLevel  string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Charset  string
}

type CollectorConfig struct {
	MountPath       string
	CollectInterval time.Duration
}

func LoadConfig() (*Config, error) {
	dbPort, err := strconv.Atoi(getEnv("DB_PORT", "3306"))
	if err != nil {
		return nil, fmt.Errorf("잘못된 DB_PORT 값: %w", err)
	}

	intervalMinutes, err := strconv.Atoi(getEnv("COLLECT_INTERVAL_MINUTES", "1"))
	if err != nil {
		return nil, fmt.Errorf("잘못된 COLLECT_INTERVAL_MINUTES 값: %w", err)
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("NODE_NAME 환경 변수가 설정되지 않았습니다")
	}

	cfg := &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "root"),
			Password: os.Getenv("DB_PASSWORD"),
			Database: getEnv("DB_NAME", "harman"),
			Charset:  "utf8mb4",
		},
		Collector: CollectorConfig{
			MountPath:       getEnv("MOUNT_PATH", "/mnt/harman"),
			CollectInterval: time.Duration(intervalMinutes) * time.Minute,
		},
		NodeName: nodeName,
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

func (dc *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		dc.User,
		dc.Password,
		dc.Host,
		dc.Port,
		dc.Database,
		dc.Charset,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

