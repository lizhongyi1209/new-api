package model

import (
	"errors"
	"sort"

	"github.com/QuantumNous/new-api/constant"
	"gorm.io/gorm"
)

var ErrImageGalleryImageNotFound = errors.New("image gallery image not found")

type ImageGallerySelection struct {
	TaskID string `json:"task_id"`
	Index  int    `json:"index"`
}

// DeleteImageGallerySelections hides saved images without renumbering their
// indices, so other gallery download URLs keep pointing at the same image.
func DeleteImageGallerySelections(userID int, selections []ImageGallerySelection) (int, error) {
	grouped := make(map[string]map[int]struct{})
	for _, selection := range selections {
		if selection.TaskID == "" || selection.Index < 0 {
			return 0, ErrImageGalleryImageNotFound
		}
		if grouped[selection.TaskID] == nil {
			grouped[selection.TaskID] = make(map[int]struct{})
		}
		grouped[selection.TaskID][selection.Index] = struct{}{}
	}

	taskIDs := make([]string, 0, len(grouped))
	for taskID := range grouped {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)

	deleted := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, taskID := range taskIDs {
			var task Task
			err := lockForUpdate(tx).Omit("data").
				Where("user_id = ? AND task_id = ? AND platform = ? AND status = ?", userID, taskID, constant.TaskPlatformGenerateImage, TaskStatusSuccess).
				First(&task).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrImageGalleryImageNotFound
			}
			if err != nil {
				return err
			}
			if !task.PrivateData.ImageGallery {
				return ErrImageGalleryImageNotFound
			}
			for index := range grouped[taskID] {
				if index >= len(task.PrivateData.ImageGalleryResults) ||
					task.PrivateData.ImageGalleryResults[index].URL == "" {
					return ErrImageGalleryImageNotFound
				}
			}
			for index := range grouped[taskID] {
				task.PrivateData.ImageGalleryResults[index].URL = ""
				deleted++
			}
			task.PrivateData.ResultURL = ""
			for _, image := range task.PrivateData.ImageGalleryResults {
				if image.URL != "" {
					task.PrivateData.ResultURL = image.URL
					break
				}
			}
			if err := tx.Model(&task).Updates(map[string]any{
				"private_data": task.PrivateData,
				"data":         nil,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
