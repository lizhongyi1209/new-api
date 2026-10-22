package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type SeedanceAssetRecoveryAdaptor interface {
	MissingSeedanceAsset(task *model.Task, reason string) bool
	RecreateMissingSeedanceAsset(ctx context.Context, task *model.Task, channel *model.Channel, reason string) (upstreamID string, responseBody []byte, submitted bool, err error)
}

func recoverMissingSeedanceAsset(ctx context.Context, adaptor TaskPollingAdaptor, channel *model.Channel, task *model.Task, result *relaycommon.TaskInfo) (bool, error) {
	recovery := task.PrivateData.SeedanceAssetRecovery
	worker, ok := adaptor.(SeedanceAssetRecoveryAdaptor)
	if !ok || recovery == nil || result.Status != model.TaskStatusFailure || !worker.MissingSeedanceAsset(task, result.Reason) {
		return false, nil
	}
	if recovery.StartedAt == 0 && (recovery.Stopped || recovery.Attempts >= model.SeedanceAssetRecoveryLimit) {
		return false, nil
	}
	expectedUpstreamID := task.GetUpstreamTaskID()
	claimed := false
	_, err := model.UpdateSeedanceRecoveryTask(ctx, task, func(current *model.Task) bool {
		state := current.PrivateData.SeedanceAssetRecovery
		if state.StartedAt != 0 {
			if time.Now().Unix()-state.StartedAt <= 5*60 {
				return false
			}
			// A crashed worker may already have submitted a generation. Stop
			// rather than duplicate an upstream request with an unknown outcome.
			state.StartedAt = 0
			state.Stopped = true
			return true
		}
		if state.Stopped || state.Attempts >= model.SeedanceAssetRecoveryLimit {
			return false
		}
		state.Attempts++
		state.StartedAt = time.Now().Unix()
		current.Status = model.TaskStatusInProgress
		current.Progress = "0%"
		current.Data = nil
		current.FailReason = ""
		claimed = true
		return true
	})
	if err != nil {
		return true, err
	}
	if !claimed {
		if task.PrivateData.SeedanceAssetRecovery == nil {
			return true, nil
		}
		return task.GetUpstreamTaskID() != expectedUpstreamID || task.PrivateData.SeedanceAssetRecovery.StartedAt != 0 ||
			task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure, nil
	}

	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	upstreamID, responseBody, submitted, recoveryErr := worker.RecreateMissingSeedanceAsset(recoveryCtx, task, channel, result.Reason)
	state := task.PrivateData.SeedanceAssetRecovery
	// Preserve the mutated replay request (new asset ID) when committing the
	// new upstream ID. The public task and its quota reservation stay intact.
	updated, err := model.UpdateSeedanceRecoveryTask(ctx, task, func(current *model.Task) bool {
		current.PrivateData.SeedanceAssetRecovery = state
		state.StartedAt = 0
		if submitted {
			current.PrivateData.UsedChannels = append(current.PrivateData.UsedChannels, strconv.Itoa(channel.Id))
		}
		if recoveryErr != nil {
			state.Stopped = submitted
			return true
		}
		current.PrivateData.UpstreamTaskID = upstreamID
		current.Status = model.TaskStatusQueued
		current.Progress = "0%"
		current.StartTime = 0
		current.FinishTime = 0
		current.FailReason = ""
		current.Data = redactVideoResponseBody(responseBody)
		return true
	})
	if err != nil {
		return true, err
	}
	if !updated {
		return true, nil
	}
	model.UpdateConsumeLogOther(task.PrivateData.SubmitLogID, map[string]interface{}{
		"admin_info": map[string]interface{}{
			"use_channel":             task.PrivateData.UsedChannels,
			"seedance_asset_recovery": map[string]interface{}{"attempts": state.Attempts, "limit": model.SeedanceAssetRecoveryLimit},
		},
	})
	logger.LogInfo(ctx, fmt.Sprintf("seedance asset recovery: task=%s channel=%d attempt=%d limit=%d succeeded=%t submitted=%t",
		task.TaskID, channel.Id, state.Attempts, model.SeedanceAssetRecoveryLimit, recoveryErr == nil, submitted))
	if recoveryErr != nil {
		return !state.Stopped && state.Attempts < model.SeedanceAssetRecoveryLimit, nil
	}
	return true, nil
}
