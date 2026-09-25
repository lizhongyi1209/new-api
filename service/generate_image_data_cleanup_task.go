package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	generateImageDataCleanupTickInterval = 24 * time.Hour
	generateImageDataCleanupBatchSize    = 500
	// 默认保留 24 小时：客户端异步轮询取图后，base64 即成为死数据。
	generateImageDataDefaultRetentionHours = 24
	// 持久化的清理水位：已清理到的 finish_time（秒）。下次只处理该时刻之后、
	// 新 cutoff 之前的窗口，避免重复 UPDATE 同一行造成 MVCC 死元组堆积。
	generateImageDataCleanupWatermarkOption = "GenerateImageDataCleanupWatermark"
	// async_image 的清理水位。两者筛选维度不同（platform vs platform+action），
	// 共用水位会让其中一个漏扫，因此各自独立推进。
	asyncImageDataCleanupWatermarkOption = "AsyncImageDataCleanupWatermark"
)

var (
	generateImageDataCleanupOnce    sync.Once
	generateImageDataCleanupRunning atomic.Bool
)

// generateImageDataRetentionHours 读取保留窗口（小时），可经环境变量
// GENERATE_IMAGE_DATA_RETENTION_HOURS 调整。<=0 视为关闭自动清理。
func generateImageDataRetentionHours() int {
	return common.GetEnvOrDefault("GENERATE_IMAGE_DATA_RETENTION_HOURS", generateImageDataDefaultRetentionHours)
}

// StartGenerateImageDataCleanupTask 启动后台任务，定期清空已过保留期的
// generate_image 与 async_image 任务 `data` 列中的 base64 图片数据（保留任务
// 行本身）。仅主节点执行，避免多副本重复清理。
func StartGenerateImageDataCleanupTask() {
	generateImageDataCleanupOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if generateImageDataRetentionHours() <= 0 {
			logger.LogInfo(context.Background(), "task data cleanup disabled (retention hours <= 0)")
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf(
				"task data cleanup started: tick=%s, retention=%dh",
				generateImageDataCleanupTickInterval, generateImageDataRetentionHours()))
			ticker := time.NewTicker(generateImageDataCleanupTickInterval)
			defer ticker.Stop()

			runGenerateImageDataCleanupOnce()
			for range ticker.C {
				runGenerateImageDataCleanupOnce()
			}
		})
	})
}

func runGenerateImageDataCleanupOnce() {
	if !generateImageDataCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer generateImageDataCleanupRunning.Store(false)

	retentionHours := generateImageDataRetentionHours()
	if retentionHours <= 0 {
		return
	}

	ctx := context.Background()
	cutoffUnix := time.Now().Add(-time.Duration(retentionHours) * time.Hour).Unix()

	runGenerateImageDataCleanupWindow(ctx, cutoffUnix)
	runAsyncImageDataCleanupWindow(ctx, cutoffUnix)
}

func runGenerateImageDataCleanupWindow(ctx context.Context, cutoffUnix int64) {
	since := loadCleanupWatermark(generateImageDataCleanupWatermarkOption)
	if since >= cutoffUnix {
		return // 窗口为空：上次已清理到 cutoff 之后，无新行可清。
	}

	cleared, err := model.ClearGenerateImageDataWindow(since, cutoffUnix, generateImageDataCleanupBatchSize)
	if err != nil {
		// 出错不推进水位：下次重试同一窗口（清理本身幂等，重清已空行无害）。
		logger.LogWarn(ctx, fmt.Sprintf("generate_image data cleanup failed (window %d-%d, cleared %d before error): %v",
			since, cutoffUnix, cleared, err))
		return
	}

	storeCleanupWatermark(ctx, "generate_image", generateImageDataCleanupWatermarkOption, cutoffUnix)
	if cleared > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("generate_image data cleanup: cleared base64 from %d task(s), watermark advanced to %s",
			cleared, time.Unix(cutoffUnix, 0).Format(time.RFC3339)))
	}
}

func runAsyncImageDataCleanupWindow(ctx context.Context, cutoffUnix int64) {
	since := loadCleanupWatermark(asyncImageDataCleanupWatermarkOption)
	if since >= cutoffUnix {
		return
	}

	cleared, err := model.ClearAsyncImageDataWindow(since, cutoffUnix, generateImageDataCleanupBatchSize)
	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("async_image data cleanup failed (window %d-%d, cleared %d before error): %v",
			since, cutoffUnix, cleared, err))
		return
	}

	storeCleanupWatermark(ctx, "async_image", asyncImageDataCleanupWatermarkOption, cutoffUnix)
	if cleared > 0 {
		logger.LogInfo(ctx, fmt.Sprintf("async_image data cleanup: cleared base64 from %d task(s), watermark advanced to %s",
			cleared, time.Unix(cutoffUnix, 0).Format(time.RFC3339)))
	}
}

// loadCleanupWatermark 读取已清理到的 finish_time 水位。
// 缺省（首次运行）返回 0，表示从最早的任务开始清理历史积压。
func loadCleanupWatermark(option string) int64 {
	common.OptionMapRWMutex.RLock()
	raw := common.OptionMap[option]
	common.OptionMapRWMutex.RUnlock()
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func storeCleanupWatermark(ctx context.Context, label string, option string, cutoffUnix int64) {
	if err := model.UpdateOption(option, strconv.FormatInt(cutoffUnix, 10)); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("%s data cleanup: failed to persist watermark: %v", label, err))
	}
}
