package fallback

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/songquanpeng/one-api/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var raceTestDBCounter int64

func setupRaceTestDB(t *testing.T) func() {
	t.Helper()
	originalDB := model.DB
	dbName := fmt.Sprintf("file:race_test_%s_%d?mode=memory&cache=shared", t.Name(), atomic.AddInt64(&raceTestDBCounter, 1))
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	return func() {
		model.DB = originalDB
	}
}

func TestRaceDeploymentUsageAndErrorCounters(t *testing.T) {
	cleanupDB := setupRaceTestDB(t)
	defer cleanupDB()

	if err := InitStateStore(); err != nil {
		t.Fatalf("InitStateStore failed: %v", err)
	}

	deploymentID := "test:deployment-race-001"
	usage := UsageInfo{TotalTokens: 1}

	// 100 RecordDeploymentUsage calls from 10 goroutines
	var usageWg sync.WaitGroup
	usageErrs := make(chan error, 100)
	for i := 0; i < 10; i++ {
		usageWg.Add(1)
		go func() {
			defer usageWg.Done()
			for j := 0; j < 10; j++ {
				if err := RecordDeploymentUsage(deploymentID, usage); err != nil {
					usageErrs <- err
				}
			}
		}()
	}
	usageWg.Wait()
	close(usageErrs)
	for err := range usageErrs {
		if err != nil {
			t.Fatalf("RecordDeploymentUsage failed: %v", err)
		}
	}

	state, err := GetDeploymentState(deploymentID, todayString())
	if err != nil {
		t.Fatalf("GetDeploymentState failed after usage: %v", err)
	}
	if state.UsedTotalTokens != 100 {
		t.Fatalf("expected UsedTotalTokens=100, got %d", state.UsedTotalTokens)
	}
	if state.RequestCount != 100 {
		t.Fatalf("expected RequestCount=100 after usage, got %d", state.RequestCount)
	}

	// 50 RecordDeploymentError calls from 5 goroutines
	var errorWg sync.WaitGroup
	errorErrs := make(chan error, 50)
	for i := 0; i < 5; i++ {
		errorWg.Add(1)
		go func() {
			defer errorWg.Done()
			for j := 0; j < 10; j++ {
				if err := RecordDeploymentError(deploymentID, errors.New("test error")); err != nil {
					errorErrs <- err
				}
			}
		}()
	}
	errorWg.Wait()
	close(errorErrs)
	for err := range errorErrs {
		if err != nil {
			t.Fatalf("RecordDeploymentError failed: %v", err)
		}
	}

	state, err = GetDeploymentState(deploymentID, todayString())
	if err != nil {
		t.Fatalf("GetDeploymentState failed after errors: %v", err)
	}
	if state.ErrorCount != 50 {
		t.Fatalf("expected ErrorCount=50, got %d", state.ErrorCount)
	}
	if state.RequestCount != 150 {
		t.Fatalf("expected RequestCount=150, got %d", state.RequestCount)
	}
}
