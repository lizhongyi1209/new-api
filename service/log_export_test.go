package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcquireLogExportLockIsScopedByUser(t *testing.T) {
	for _, useRedis := range []bool{false, true} {
		t.Run(fmt.Sprintf("redis=%v", useRedis), func(t *testing.T) {
			previousRedis, previousClient := common.RedisEnabled, common.RDB
			common.RedisEnabled = useRedis
			if useRedis {
				server := miniredis.RunT(t)
				common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
				require.NoError(t, common.RDB.Ping(context.Background()).Err())
			}
			t.Cleanup(func() {
				common.RedisEnabled, common.RDB = previousRedis, previousClient
			})

			release, ok, err := AcquireLogExportLock(1)
			require.NoError(t, err)
			require.True(t, ok)

			_, ok, err = AcquireLogExportLock(1)
			require.NoError(t, err)
			assert.False(t, ok)

			releaseOther, ok, err := AcquireLogExportLock(2)
			require.NoError(t, err)
			require.True(t, ok)
			releaseOther()

			release()
			releaseAgain, ok, err := AcquireLogExportLock(1)
			require.NoError(t, err)
			require.True(t, ok)
			releaseAgain()
		})
	}
}
