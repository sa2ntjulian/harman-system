package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jeongdaeha/harman-k8s-collector/internal/config"
	"github.com/jeongdaeha/harman-k8s-collector/internal/models"
	"github.com/jeongdaeha/harman-k8s-collector/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Collector struct {
	db                  *gorm.DB
	config              *config.Config
	collectionCount     metric.Int64Counter
	collectionFiles     metric.Int64UpDownCounter
	collectionDuration  metric.Float64Histogram
	dbOperationDuration metric.Float64Histogram
	collectionErrors    metric.Int64Counter
}

func NewCollector(db *gorm.DB, cfg *config.Config) (*Collector, error) {
	meter := telemetry.Meter()
	if meter == nil {
		return nil, fmt.Errorf("메터가 초기화되지 않았습니다")
	}

	collectionCount, err := meter.Int64Counter(
		"file_collection_count",
		metric.WithDescription("파일 수집 횟수"),
	)
	if err != nil {
		return nil, fmt.Errorf("collection_count 메트릭 생성 실패: %w", err)
	}

	collectionFiles, err := meter.Int64UpDownCounter(
		"file_collection_files",
		metric.WithDescription("수집된 파일 수"),
	)
	if err != nil {
		return nil, fmt.Errorf("collection_files 메트릭 생성 실패: %w", err)
	}

	collectionDuration, err := meter.Float64Histogram(
		"file_collection_duration_ms",
		metric.WithDescription("파일 수집 소요 시간 (밀리초)"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("collection_duration 메트릭 생성 실패: %w", err)
	}

	dbOperationDuration, err := meter.Float64Histogram(
		"db_operation_duration_ms",
		metric.WithDescription("데이터베이스 작업 소요 시간 (밀리초)"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("db_operation_duration 메트릭 생성 실패: %w", err)
	}

	collectionErrors, err := meter.Int64Counter(
		"file_collection_errors",
		metric.WithDescription("파일 수집 에러 발생 횟수"),
	)
	if err != nil {
		return nil, fmt.Errorf("collection_errors 메트릭 생성 실패: %w", err)
	}

	return &Collector{
		db:                  db,
		config:              cfg,
		collectionCount:     collectionCount,
		collectionFiles:     collectionFiles,
		collectionDuration:  collectionDuration,
		dbOperationDuration: dbOperationDuration,
		collectionErrors:    collectionErrors,
	}, nil
}

func (c *Collector) Start(ctx context.Context) error {
	slog.Info("파일 수집기 시작",
		"node_name", c.config.NodeName,
		"mount_path", c.config.Collector.MountPath,
		"interval", c.config.Collector.CollectInterval.String(),
	)

	if err := c.collectAndStore(); err != nil {
		slog.Error("초기 파일 수집 실패", "error", err)
	}

	ticker := time.NewTicker(c.config.Collector.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("파일 수집기 종료 신호 수신")
			return ctx.Err()
		case <-ticker.C:
			if err := c.collectAndStore(); err != nil {
				slog.Error("파일 수집 실패", "error", err)
			}
		}
	}
}

func (c *Collector) collectAndStore() error {
	ctx := context.Background()
	tracer := telemetry.Tracer()
	if tracer == nil {
		return c.collectAndStoreWithoutTrace(ctx)
	}

	ctx, span := tracer.Start(ctx, "file_collection",
		oteltrace.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
			attribute.String("mount_path", c.config.Collector.MountPath),
		),
	)
	defer span.End()

	startTime := time.Now()

	_, scanSpan := tracer.Start(ctx, "scan_directory")
	files, err := c.scanDirectory(c.config.Collector.MountPath)
	scanSpan.End()
	if err != nil {
		c.collectionErrors.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("error_type", "scan_error"),
				attribute.String("node_name", c.config.NodeName),
			),
		)
		span.RecordError(err)
		return fmt.Errorf("디렉토리 스캔 실패: %w", err)
	}

	fileCount := len(files)
	scanSpan.SetAttributes(
		attribute.Int("file_count", fileCount),
	)

	slog.Info("파일 스캔 완료",
		"node_name", c.config.NodeName,
		"mount_path", c.config.Collector.MountPath,
		"file_count", fileCount,
	)

	_, saveSpan := tracer.Start(ctx, "save_to_database")
	dbStartTime := time.Now()
	if err := c.saveToDatabase(files); err != nil {
		c.collectionErrors.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("error_type", "db_error"),
				attribute.String("node_name", c.config.NodeName),
			),
		)
		saveSpan.RecordError(err)
		saveSpan.End()
		span.RecordError(err)
		return fmt.Errorf("데이터베이스 저장 실패: %w", err)
	}
	dbDuration := time.Since(dbStartTime)
	saveSpan.SetAttributes(
		attribute.Int("file_count", fileCount),
		attribute.Int64("db_duration_ms", dbDuration.Milliseconds()),
	)
	saveSpan.End()

	duration := time.Since(startTime)
	c.collectionCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.collectionFiles.Add(ctx, int64(fileCount),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.collectionDuration.Record(ctx, float64(duration.Milliseconds()),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.dbOperationDuration.Record(ctx, float64(dbDuration.Milliseconds()),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)

	span.SetAttributes(
		attribute.Int("file_count", fileCount),
		attribute.Int64("duration_ms", duration.Milliseconds()),
	)

	slog.Info("파일 수집 완료",
		"node_name", c.config.NodeName,
		"file_count", fileCount,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (c *Collector) collectAndStoreWithoutTrace(ctx context.Context) error {
	startTime := time.Now()

	files, err := c.scanDirectory(c.config.Collector.MountPath)
	if err != nil {
		c.collectionErrors.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("error_type", "scan_error"),
				attribute.String("node_name", c.config.NodeName),
			),
		)
		return fmt.Errorf("디렉토리 스캔 실패: %w", err)
	}

	fileCount := len(files)
	slog.Info("파일 스캔 완료",
		"node_name", c.config.NodeName,
		"mount_path", c.config.Collector.MountPath,
		"file_count", fileCount,
	)

	dbStartTime := time.Now()
	if err := c.saveToDatabase(files); err != nil {
		c.collectionErrors.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("error_type", "db_error"),
				attribute.String("node_name", c.config.NodeName),
			),
		)
		return fmt.Errorf("데이터베이스 저장 실패: %w", err)
	}
	dbDuration := time.Since(dbStartTime)

	duration := time.Since(startTime)
	c.collectionCount.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.collectionFiles.Add(ctx, int64(fileCount),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.collectionDuration.Record(ctx, float64(duration.Milliseconds()),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)
	c.dbOperationDuration.Record(ctx, float64(dbDuration.Milliseconds()),
		metric.WithAttributes(
			attribute.String("node_name", c.config.NodeName),
		),
	)

	slog.Info("파일 수집 완료",
		"node_name", c.config.NodeName,
		"file_count", fileCount,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (c *Collector) scanDirectory(dirPath string) ([]string, error) {
	var files []string

	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("디렉토리가 존재하지 않습니다", "path", dirPath)
			return files, nil
		}
		return nil, fmt.Errorf("디렉토리 정보 조회 실패: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("지정된 경로가 디렉토리가 아닙니다: %s", dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("디렉토리 읽기 실패: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

func (c *Collector) saveToDatabase(files []string) error {
	return c.db.Transaction(func(tx *gorm.DB) error {
		collection := models.FileCollection{
			NodeName:    c.config.NodeName,
			MountPath:   c.config.Collector.MountPath,
			CollectedAt: time.Now(),
			FileCount:   len(files),
		}

		if err := tx.Create(&collection).Error; err != nil {
			return fmt.Errorf("file_collections 삽입 실패: %w", err)
		}

		if len(files) > 0 {
			collectedFiles := make([]models.CollectedFile, 0, len(files))
			for _, fileName := range files {
				collectedFiles = append(collectedFiles, models.CollectedFile{
					CollectionID: collection.ID,
					FileName:     fileName,
				})
			}

			if err := tx.CreateInBatches(collectedFiles, 100).Error; err != nil {
				return fmt.Errorf("collected_files 배치 삽입 실패: %w", err)
			}
		}

		slog.Debug("데이터베이스 저장 성공",
			"collection_id", collection.ID,
			"file_count", len(files),
		)

		return nil
	})
}

func (c *Collector) GetMountPath() string {
	return filepath.Clean(c.config.Collector.MountPath)
}

