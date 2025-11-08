package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jeongdaeha/harman-k8s-collector/internal/collector"
	"github.com/jeongdaeha/harman-k8s-collector/internal/config"
	"github.com/jeongdaeha/harman-k8s-collector/internal/database"
	"github.com/jeongdaeha/harman-k8s-collector/internal/logger"
	"github.com/jeongdaeha/harman-k8s-collector/internal/telemetry"
)

func main() {
	logger.InitLogger("info")

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("설정 로드 실패", "error", err)
		os.Exit(1)
	}

	logger.InitLogger(cfg.LogLevel)

	slog.Info("설정 로드 완료",
		"node_name", cfg.NodeName,
		"mount_path", cfg.Collector.MountPath,
		"collect_interval", cfg.Collector.CollectInterval.String(),
		"db_host", cfg.Database.Host,
	)

	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		slog.Error("데이터베이스 연결 실패", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.CloseDB(db); err != nil {
			slog.Error("데이터베이스 연결 종료 실패", "error", err)
		}
	}()

	if err := database.MigrateDB(db); err != nil {
		slog.Error("데이터베이스 마이그레이션 실패", "error", err)
		os.Exit(1)
	}

	if err := telemetry.InitTelemetry("file-collector", cfg.NodeName); err != nil {
		slog.Warn("OpenTelemetry 초기화 실패", "error", err)
	} else {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := telemetry.Shutdown(ctx); err != nil {
				slog.Error("OpenTelemetry 종료 실패", "error", err)
			}
		}()
	}

	fileCollector, err := collector.NewCollector(db, cfg)
	if err != nil {
		slog.Error("파일 수집기 생성 실패", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- fileCollector.Start(ctx)
	}()

	select {
	case sig := <-sigChan:
		slog.Info("종료 신호 수신", "signal", sig.String())
		cancel()
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			slog.Error("수집기 에러 발생", "error", err)
		}
	}

	slog.Info("파일 수집기 종료")
}

