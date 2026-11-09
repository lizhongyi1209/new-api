package model

import "github.com/QuantumNous/new-api/constant"

// Kling40TaskActions are the persisted actions for Kling 4.0 and 4.0 Flash.
func Kling40TaskActions() []string {
	return []string{
		constant.TaskActionKling40Text,
		constant.TaskActionKling40Image,
		constant.TaskActionKling40Omni,
	}
}

func KlingExternalTaskExists(userID int, externalID string) (bool, error) {
	if userID <= 0 || externalID == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&Task{}).
		Where("user_id = ? AND external_task_id = ? AND action IN ?", userID, externalID, Kling40TaskActions()).
		Count(&count).Error
	return count > 0, err
}

// Kling40TaskQuery selects one user's Kling 4.0 / Flash tasks.
// CursorID, when set, returns rows with a smaller primary key.
type Kling40TaskQuery struct {
	TaskIDs     []string
	ExternalIDs []string
	Statuses    []TaskStatus
	StartSec    int64
	EndSec      int64
	UseTime     bool
	CursorID    int64
	Limit       int
}

func ListUserKling40Tasks(userID int, query Kling40TaskQuery) ([]*Task, error) {
	if userID <= 0 {
		return nil, nil
	}
	tx := DB.Where("user_id = ? AND action IN ?", userID, Kling40TaskActions())
	if len(query.TaskIDs) > 0 {
		tx = tx.Where("task_id IN ?", query.TaskIDs)
	}
	if len(query.ExternalIDs) > 0 {
		tx = tx.Where("external_task_id IN ?", query.ExternalIDs)
	}
	if len(query.Statuses) > 0 {
		tx = tx.Where("status IN ?", query.Statuses)
	}
	if query.UseTime {
		if query.StartSec > 0 {
			tx = tx.Where("submit_time >= ?", query.StartSec)
		}
		if query.EndSec > 0 {
			tx = tx.Where("submit_time <= ?", query.EndSec)
		}
	}
	if query.CursorID > 0 {
		tx = tx.Where("id < ?", query.CursorID)
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	var tasks []*Task
	err := tx.Order("id DESC").Limit(limit).Find(&tasks).Error
	return tasks, err
}
