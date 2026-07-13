package fallback

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	dbmodel "github.com/songquanpeng/one-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type freeProviderCatalogRecord struct {
	ID            int        `gorm:"primaryKey"`
	DeploymentID  string     `gorm:"size:191;uniqueIndex"`
	Provider      string     `gorm:"size:64;index"`
	Source        string     `gorm:"size:64"`
	ModelsJSON    string     `gorm:"type:text"`
	SelectedModel string     `gorm:"size:191"`
	LastAttemptAt *time.Time `gorm:"column:last_attempt_at"`
	LastSuccessAt *time.Time `gorm:"column:last_success_at"`
	LastError     string     `gorm:"type:text"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

// FreeProviderCatalogSnapshot is the last successfully validated catalog plus
// independent refresh diagnostics for one automatic free deployment.
type FreeProviderCatalogSnapshot struct {
	DeploymentID  string                  `json:"deployment_id"`
	Provider      string                  `json:"provider"`
	Source        string                  `json:"source"`
	Models        []FreeModelCatalogEntry `json:"models"`
	SelectedModel string                  `json:"selected_model"`
	LastAttemptAt time.Time               `json:"last_attempt_at"`
	LastSuccessAt time.Time               `json:"last_success_at"`
	LastError     string                  `json:"last_error,omitempty"`
}

var freeProviderCatalogStore = struct {
	sync.RWMutex
	activeDB  *gorm.DB
	snapshots map[string]FreeProviderCatalogSnapshot
}{snapshots: map[string]FreeProviderCatalogSnapshot{}}

var freeProviderCatalogWriteMu sync.Mutex

func InitFreeProviderCatalogStore() error {
	db := dbmodel.DB
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}

	freeProviderCatalogStore.Lock()
	defer freeProviderCatalogStore.Unlock()
	if freeProviderCatalogStore.activeDB == db {
		return nil
	}
	if err := db.AutoMigrate(&freeProviderCatalogRecord{}); err != nil {
		return err
	}

	var records []freeProviderCatalogRecord
	if err := db.Find(&records).Error; err != nil {
		return err
	}
	snapshots := make(map[string]FreeProviderCatalogSnapshot, len(records))
	for _, record := range records {
		snapshot, err := catalogSnapshotFromRecord(record)
		if err != nil {
			return fmt.Errorf("decode catalog snapshot %s: %w", record.DeploymentID, err)
		}
		snapshots[record.DeploymentID] = snapshot
	}
	freeProviderCatalogStore.activeDB = db
	freeProviderCatalogStore.snapshots = snapshots
	return nil
}

func saveFreeProviderCatalogSuccess(snapshot FreeProviderCatalogSnapshot) error {
	if err := InitFreeProviderCatalogStore(); err != nil {
		return err
	}
	validated, err := normalizeFreeProviderCatalogSnapshot(snapshot)
	if err != nil {
		return err
	}
	snapshot = validated
	freeProviderCatalogWriteMu.Lock()
	defer freeProviderCatalogWriteMu.Unlock()
	if err := dbmodel.DB.Transaction(func(tx *gorm.DB) error {
		return saveFreeProviderCatalogSuccessWithDB(tx, snapshot)
	}); err != nil {
		return err
	}
	cacheFreeProviderCatalogSnapshot(snapshot)
	return nil
}

func saveFreeProviderCatalogSuccessWithDB(tx *gorm.DB, snapshot FreeProviderCatalogSnapshot) error {
	if tx == nil {
		return fmt.Errorf("database is not initialized")
	}
	snapshot.DeploymentID = strings.TrimSpace(snapshot.DeploymentID)
	snapshot.Provider = strings.TrimSpace(snapshot.Provider)
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	snapshot.SelectedModel = strings.TrimSpace(snapshot.SelectedModel)
	if snapshot.DeploymentID == "" || snapshot.Provider == "" {
		return fmt.Errorf("catalog snapshot requires deployment and provider")
	}
	validated, err := validateFreeProviderCatalog(FreeProviderCatalogCandidate{
		Source: snapshot.Source,
		Models: snapshot.Models,
	})
	if err != nil {
		return err
	}
	snapshot.Source = validated.Source
	snapshot.Models = validated.Models
	if snapshot.SelectedModel == "" {
		return fmt.Errorf("catalog snapshot requires a selected model")
	}
	modelsJSON, err := json.Marshal(snapshot.Models)
	if err != nil {
		return fmt.Errorf("encode catalog models: %w", err)
	}
	lastAttemptAt := snapshot.LastAttemptAt.UTC()
	lastSuccessAt := snapshot.LastSuccessAt.UTC()
	record := freeProviderCatalogRecord{
		DeploymentID:  snapshot.DeploymentID,
		Provider:      snapshot.Provider,
		Source:        snapshot.Source,
		ModelsJSON:    string(modelsJSON),
		SelectedModel: snapshot.SelectedModel,
		LastAttemptAt: &lastAttemptAt,
		LastSuccessAt: &lastSuccessAt,
		LastError:     "",
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "deployment_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider", "source", "models_json", "selected_model",
			"last_attempt_at", "last_success_at", "last_error", "updated_at",
		}),
	}).Create(&record).Error
}

func markFreeProviderCatalogFailure(deploymentID, provider, source string, attemptedAt time.Time, syncErr error) error {
	if syncErr == nil {
		return nil
	}
	if err := InitFreeProviderCatalogStore(); err != nil {
		return err
	}
	freeProviderCatalogWriteMu.Lock()
	defer freeProviderCatalogWriteMu.Unlock()

	attemptedAt = attemptedAt.UTC()
	errorText := strings.TrimSpace(syncErr.Error())
	var record freeProviderCatalogRecord
	err := dbmodel.DB.Where("deployment_id = ?", deploymentID).First(&record).Error
	switch {
	case err == nil:
		if err := dbmodel.DB.Model(&record).Updates(map[string]any{
			"last_attempt_at": attemptedAt,
			"last_error":      errorText,
			"source":          source,
		}).Error; err != nil {
			return err
		}
	case err == gorm.ErrRecordNotFound:
		record = freeProviderCatalogRecord{
			DeploymentID:  deploymentID,
			Provider:      provider,
			Source:        source,
			ModelsJSON:    "[]",
			LastAttemptAt: &attemptedAt,
			LastError:     errorText,
		}
		if err := dbmodel.DB.Create(&record).Error; err != nil {
			return err
		}
	default:
		return err
	}

	freeProviderCatalogStore.Lock()
	snapshot := freeProviderCatalogStore.snapshots[deploymentID]
	snapshot.DeploymentID = deploymentID
	snapshot.Provider = provider
	snapshot.Source = source
	snapshot.LastAttemptAt = attemptedAt
	snapshot.LastError = errorText
	freeProviderCatalogStore.snapshots[deploymentID] = cloneFreeProviderCatalogSnapshot(snapshot)
	freeProviderCatalogStore.Unlock()
	return nil
}

func GetFreeProviderCatalogSnapshot(deploymentID string) (FreeProviderCatalogSnapshot, bool) {
	if err := InitFreeProviderCatalogStore(); err != nil {
		return FreeProviderCatalogSnapshot{}, false
	}
	freeProviderCatalogStore.RLock()
	snapshot, ok := freeProviderCatalogStore.snapshots[deploymentID]
	freeProviderCatalogStore.RUnlock()
	if !ok {
		return FreeProviderCatalogSnapshot{}, false
	}
	return cloneFreeProviderCatalogSnapshot(snapshot), true
}

func cacheFreeProviderCatalogSnapshot(snapshot FreeProviderCatalogSnapshot) {
	snapshot.LastError = ""
	freeProviderCatalogStore.Lock()
	freeProviderCatalogStore.snapshots[snapshot.DeploymentID] = cloneFreeProviderCatalogSnapshot(snapshot)
	freeProviderCatalogStore.Unlock()
}

func normalizeFreeProviderCatalogSnapshot(snapshot FreeProviderCatalogSnapshot) (FreeProviderCatalogSnapshot, error) {
	snapshot.DeploymentID = strings.TrimSpace(snapshot.DeploymentID)
	snapshot.Provider = strings.TrimSpace(snapshot.Provider)
	snapshot.Source = strings.TrimSpace(snapshot.Source)
	snapshot.SelectedModel = strings.TrimSpace(snapshot.SelectedModel)
	if snapshot.DeploymentID == "" || snapshot.Provider == "" {
		return FreeProviderCatalogSnapshot{}, fmt.Errorf("catalog snapshot requires deployment and provider")
	}
	validated, err := validateFreeProviderCatalog(FreeProviderCatalogCandidate{
		Source: snapshot.Source,
		Models: snapshot.Models,
	})
	if err != nil {
		return FreeProviderCatalogSnapshot{}, err
	}
	snapshot.Source = validated.Source
	snapshot.Models = validated.Models
	if snapshot.SelectedModel == "" {
		return FreeProviderCatalogSnapshot{}, fmt.Errorf("catalog snapshot requires a selected model")
	}
	snapshot.LastAttemptAt = snapshot.LastAttemptAt.UTC()
	snapshot.LastSuccessAt = snapshot.LastSuccessAt.UTC()
	snapshot.LastError = ""
	return snapshot, nil
}

func catalogSnapshotFromRecord(record freeProviderCatalogRecord) (FreeProviderCatalogSnapshot, error) {
	snapshot := FreeProviderCatalogSnapshot{
		DeploymentID:  record.DeploymentID,
		Provider:      record.Provider,
		Source:        record.Source,
		SelectedModel: record.SelectedModel,
		LastError:     record.LastError,
	}
	if strings.TrimSpace(record.ModelsJSON) != "" {
		if err := json.Unmarshal([]byte(record.ModelsJSON), &snapshot.Models); err != nil {
			return FreeProviderCatalogSnapshot{}, err
		}
	}
	if record.LastAttemptAt != nil {
		snapshot.LastAttemptAt = record.LastAttemptAt.UTC()
	}
	if record.LastSuccessAt != nil {
		snapshot.LastSuccessAt = record.LastSuccessAt.UTC()
	}
	return snapshot, nil
}

func cloneFreeProviderCatalogSnapshot(src FreeProviderCatalogSnapshot) FreeProviderCatalogSnapshot {
	dst := src
	dst.Models = make([]FreeModelCatalogEntry, 0, len(src.Models))
	for _, entry := range src.Models {
		dst.Models = append(dst.Models, cloneFreeModelCatalogEntry(entry))
	}
	return dst
}

func resetFreeProviderCatalogCacheForTest() {
	freeProviderCatalogStore.Lock()
	freeProviderCatalogStore.activeDB = nil
	freeProviderCatalogStore.snapshots = map[string]FreeProviderCatalogSnapshot{}
	freeProviderCatalogStore.Unlock()
}
