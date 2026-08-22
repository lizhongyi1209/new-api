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
	now := time.Now()
	outputStats, outputErr := CleanupExpiredTemporaryOutputImages(now)
	if outputErr != nil {
		logger.LogWarn(context.Background(), "temporary image cleanup failed: "+outputErr.Error())
	}
	inputStats, inputErr := CleanupExpiredTemporaryInputAttachments(now)
	if inputErr != nil {
		logger.LogWarn(context.Background(), "temporary input cleanup failed: "+inputErr.Error())
	}
	deleted := outputStats.Deleted + inputStats.Deleted
	deletedBytes := outputStats.Bytes + inputStats.Bytes
	if deleted > 0 {
		logger.LogInfo(context.Background(), fmt.Sprintf(
			"temporary file cleanup: deleted=%d bytes=%d", deleted, deletedBytes))
	}
}
