package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/logger"

	"github.com/bytedance/gopkg/util/gopool"
)

var temporaryImageCleanupOnce sync.Once

func StartTemporaryImageCleanupTask() {
	temporaryImageCleanupOnce.Do(func() {
		gopool.Go(func() {
			runTemporaryImageCleanupOnce()
			ticker := time.NewTicker(temporaryImageCleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				runTemporaryImageCleanupOnce()
			}
		})
	})
}

func runTemporaryImageCleanupOnce() {
	stats, err := CleanupExpiredTemporaryOutputImages(time.Now())
	if err != nil {
		logger.LogWarn(context.Background(), "temporary image cleanup failed: "+err.Error())
	}
	if stats.Deleted > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf(
			"temporary image cleanup: deleted=%d bytes=%d", stats.Deleted, stats.Bytes))
	}
}
