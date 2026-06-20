package model

import (
	"errors"
	"time"
)

// AigcElement is a local registry row for a Tencent VCLM "主体" (AIGC Element).
// The subject itself lives on Tencent's side and is referenced by ElementId;
// this row records ownership (UserId = the token's main account), which channel
// minted it, and a cached status so the console/clients can list and manage
// subjects without re-querying Tencent every time.
type AigcElement struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId    int    `json:"user_id" gorm:"index"`
	TokenId   int    `json:"token_id" gorm:"index"`              // token that created it (0 = console/session)
	TokenName string `json:"token_name" gorm:"type:varchar(64)"` // snapshot of the token name
	ChannelId int    `json:"channel_id" gorm:"index"`
	// Platform marks which video model family this subject belongs to (e.g.
	// "kling", later "seedance"), so subjects from different providers can be
	// listed and filtered separately as more platforms are added.
	Platform      string `json:"platform" gorm:"type:varchar(30);index"`
	JobId         string `json:"job_id" gorm:"type:varchar(191);index"`
	ElementId     string `json:"element_id" gorm:"type:varchar(191);index"`
	Name          string `json:"name" gorm:"type:varchar(64)"`
	Description   string `json:"description" gorm:"type:varchar(255)"`
	ReferenceType string `json:"reference_type" gorm:"type:varchar(20)"`
	Provider      string `json:"provider" gorm:"type:varchar(64)"`
	Status        string `json:"status" gorm:"type:varchar(20);index"` // pending / succeed / failed
	FailReason    string `json:"fail_reason" gorm:"type:varchar(500)"`
	// FrontalImage + ReferImages capture the reference images for visual
	// management. ReferImages is a JSON array of URLs stored as TEXT for
	// cross-DB compatibility.
	FrontalImage string `json:"frontal_image" gorm:"type:varchar(1024)"`
	ReferImages  string `json:"refer_images" gorm:"type:text"`
	VideoList    string `json:"video_list" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"index"`
	UpdatedAt    int64  `json:"updated_at"`
	// Username is filled in for admin listings; not stored.
	Username string `json:"username,omitempty" gorm:"-"`
}

// AigcElementPlatformKling is the platform tag for Kling (Tencent VCLM)
// subjects. Future video providers (e.g. seedance) get their own tag so
// subjects can be listed and filtered per platform.
const AigcElementPlatformKling = "kling"

func (e *AigcElement) Insert() error {
	now := time.Now().Unix()
	e.CreatedAt = now
	e.UpdatedAt = now
	if e.Platform == "" {
		e.Platform = AigcElementPlatformKling
	}
	return DB.Create(e).Error
}

func (e *AigcElement) Update() error {
	e.UpdatedAt = time.Now().Unix()
	return DB.Model(e).Select(
		"element_id", "job_id", "name", "description", "reference_type",
		"provider", "status", "fail_reason", "updated_at",
	).Updates(e).Error
}

// GetAigcElementById fetches one element. When userId > 0 the row must belong to
// that user; pass userId <= 0 for admin access to any element.
func GetAigcElementById(id int, userId int) (*AigcElement, error) {
	if id == 0 {
		return nil, errors.New("id 为空")
	}
	var element AigcElement
	tx := DB.Where("id = ?", id)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	if err := tx.First(&element).Error; err != nil {
		return nil, err
	}
	return &element, nil
}

func DeleteAigcElementById(id int, userId int) error {
	tx := DB.Where("id = ?", id)
	if userId > 0 {
		tx = tx.Where("user_id = ?", userId)
	}
	return tx.Delete(&AigcElement{}).Error
}

// GetUserAigcElements lists a single user's elements for a platform, newest
// first.
func GetUserAigcElements(userId int, platform string, startIdx, num int) ([]*AigcElement, int64, error) {
	var elements []*AigcElement
	var total int64
	base := DB.Model(&AigcElement{}).Where("user_id = ?", userId)
	if platform != "" {
		base = base.Where("platform = ?", platform)
	}
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tx := DB.Where("user_id = ?", userId)
	if platform != "" {
		tx = tx.Where("platform = ?", platform)
	}
	err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&elements).Error
	return elements, total, err
}

// ListUserAigcElements returns all of a user's elements for a platform without
// pagination, newest first. When onlySucceed is true, only subjects that
// finished creating (status = succeed) are returned — these are the ones usable
// in a video request. Intended for the client "pick a subject" flow.
func ListUserAigcElements(userId int, platform string, onlySucceed bool) ([]*AigcElement, error) {
	var elements []*AigcElement
	tx := DB.Where("user_id = ?", userId)
	if platform != "" {
		tx = tx.Where("platform = ?", platform)
	}
	if onlySucceed {
		tx = tx.Where("status = ?", "succeed")
	}
	err := tx.Order("id desc").Find(&elements).Error
	return elements, err
}

// GetAllAigcElements lists every user's elements for a platform (admin), newest
// first, with the owner's username joined in.
func GetAllAigcElements(platform string, startIdx, num int) ([]*AigcElement, int64, error) {
	var elements []*AigcElement
	var total int64
	base := DB.Model(&AigcElement{})
	if platform != "" {
		base = base.Where("platform = ?", platform)
	}
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	tx := DB.Where("1 = 1")
	if platform != "" {
		tx = tx.Where("platform = ?", platform)
	}
	if err := tx.Order("id desc").Limit(num).Offset(startIdx).Find(&elements).Error; err != nil {
		return nil, 0, err
	}
	fillAigcElementUsernames(elements)
	return elements, total, nil
}

// fillAigcElementUsernames populates the transient Username field for a batch of
// elements using a single id->name lookup.
func fillAigcElementUsernames(elements []*AigcElement) {
	if len(elements) == 0 {
		return
	}
	ids := make([]int, 0, len(elements))
	for _, e := range elements {
		ids = append(ids, e.UserId)
	}
	type row struct {
		Id       int
		Username string
	}
	var rows []row
	if err := DB.Model(&User{}).Select("id, username").Where("id in ?", ids).Find(&rows).Error; err != nil {
		return
	}
	nameById := make(map[int]string, len(rows))
	for _, r := range rows {
		nameById[r.Id] = r.Username
	}
	for _, e := range elements {
		e.Username = nameById[e.UserId]
	}
}
