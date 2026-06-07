package fallback

import (
	"fmt"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

type SwitchEvent struct {
	ID             int       `json:"id" gorm:"primaryKey"`
	CreatedAt      time.Time `json:"created_at" gorm:"index"`
	VirtualModel   string    `json:"virtual_model" gorm:"index"`
	FromDeployment string    `json:"from_deployment" gorm:"index"`
	ToDeployment   string    `json:"to_deployment" gorm:"index"`
	Reason         string    `json:"reason" gorm:"type:text"`
	StatusCode     int       `json:"status_code"`
	DurationMs     int64     `json:"duration_ms"`
	RequestID      string    `json:"request_id" gorm:"index"`
}

var switchEventStoreOnce sync.Once
var switchEventStoreErr error

func InitSwitchEventStore() error {
	switchEventStoreOnce.Do(func() {
		switchEventStoreErr = model.DB.AutoMigrate(&SwitchEvent{})
	})
	return switchEventStoreErr
}

func RecordSwitchEvent(event SwitchEvent) error {
	if err := InitSwitchEventStore(); err != nil {
		return err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := model.DB.Create(&event).Error; err != nil {
		logger.SysError(fmt.Sprintf("[fallback] failed to record switch event: %v", err))
		return err
	}
	return nil
}

func GetSwitchEvents(limit int) ([]SwitchEvent, error) {
	if err := InitSwitchEventStore(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	events := make([]SwitchEvent, 0)
	err := model.DB.Order("created_at desc, id desc").Limit(limit).Find(&events).Error
	return events, err
}
