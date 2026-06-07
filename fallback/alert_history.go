package fallback

import (
	"fmt"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

type AlertHistoryEvent struct {
	ID           int       `json:"id" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	DeploymentID string    `json:"deployment_id" gorm:"index"`
	Level        string    `json:"level" gorm:"index"`
	Type         string    `json:"type" gorm:"index"`
	Message      string    `json:"message" gorm:"type:text"`
	UsedTokens   int64     `json:"used_tokens"`
	DailyLimit   int64     `json:"daily_limit"`
	Percentage   float64   `json:"percentage"`
}

var alertHistoryStoreOnce sync.Once
var alertHistoryStoreErr error

func InitAlertHistoryStore() error {
	alertHistoryStoreOnce.Do(func() {
		alertHistoryStoreErr = model.DB.AutoMigrate(&AlertHistoryEvent{})
	})
	return alertHistoryStoreErr
}

func RecordAlertEvent(event AlertEvent) error {
	if err := InitAlertHistoryStore(); err != nil {
		return err
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	record := AlertHistoryEvent{
		CreatedAt:    event.CreatedAt.UTC(),
		DeploymentID: event.DeploymentID,
		Level:        string(event.Level),
		Type:         string(event.Type),
		Message:      event.Message,
		UsedTokens:   event.UsedTokens,
		DailyLimit:   event.DailyLimit,
		Percentage:   event.Percentage,
	}
	if err := model.DB.Create(&record).Error; err != nil {
		logger.SysError(fmt.Sprintf("[alert] failed to record alert history: %v", err))
		return err
	}
	return nil
}

func GetAlertHistory(limit int) ([]AlertHistoryEvent, error) {
	if err := InitAlertHistoryStore(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	events := make([]AlertHistoryEvent, 0)
	err := model.DB.Order("created_at desc, id desc").Limit(limit).Find(&events).Error
	return events, err
}
