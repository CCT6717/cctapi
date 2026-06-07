package fallback

import (
	"fmt"
	"sync"
	"time"

	"github.com/songquanpeng/one-api/model"
)

type ScoreSnapshot struct {
	ID           int       `json:"id" gorm:"primaryKey"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
	VirtualModel string    `json:"virtual_model" gorm:"index"`
	DeploymentID string    `json:"deployment_id" gorm:"index"`
	Score        float64   `json:"score"`
}

var scoreHistoryStoreOnce sync.Once
var scoreHistoryStoreErr error

func InitScoreHistoryStore() error {
	scoreHistoryStoreOnce.Do(func() {
		if model.DB == nil {
			scoreHistoryStoreErr = fmt.Errorf("database is not initialized")
			return
		}
		scoreHistoryStoreErr = model.DB.AutoMigrate(&ScoreSnapshot{})
	})
	return scoreHistoryStoreErr
}

func RecordScoreSnapshots(virtualModel string, scores map[string]float64) error {
	if len(scores) == 0 {
		return nil
	}
	if model.DB == nil {
		return nil
	}
	if err := InitScoreHistoryStore(); err != nil {
		return err
	}

	now := time.Now().UTC()
	snapshots := make([]ScoreSnapshot, 0, len(scores))
	for deploymentID, score := range scores {
		snapshots = append(snapshots, ScoreSnapshot{
			CreatedAt:    now,
			VirtualModel: virtualModel,
			DeploymentID: deploymentID,
			Score:        score,
		})
	}
	return model.DB.Create(&snapshots).Error
}

func GetScoreHistory(limit int) ([]ScoreSnapshot, error) {
	if err := InitScoreHistoryStore(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 300
	}
	if limit > 1000 {
		limit = 1000
	}

	records := make([]ScoreSnapshot, 0)
	err := model.DB.Order("created_at desc, id desc").Limit(limit).Find(&records).Error
	return records, err
}
