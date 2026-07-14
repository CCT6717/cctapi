package model

import (
	"sync"
	"testing"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateInitialRootPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "missing", wantErr: true},
		{name: "legacy default", password: "123456", wantErr: true},
		{name: "too short", password: "short-pass", wantErr: true},
		{name: "four multibyte characters are still short", password: "安全口令", wantErr: true},
		{name: "minimum length", password: "strong-pass1"},
		{name: "twelve multibyte characters", password: "这是十二个字符的安全口令值"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInitialRootPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateInitialRootPassword() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestCreateRootAccountIfNeedRequiresConfiguredPasswordForEmptyDatabase(t *testing.T) {
	withRootAccountTestDatabase(t)
	config.InitialRootPassword = ""

	if err := CreateRootAccountIfNeed(); err == nil {
		t.Fatal("expected empty database bootstrap to reject a missing password")
	}

	var count int64
	if err := DB.Model(&User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("created %d users after rejected bootstrap", count)
	}
}

func TestCreateRootAccountIfNeedUsesConfiguredPassword(t *testing.T) {
	withRootAccountTestDatabase(t)
	config.InitialRootPassword = "strong-pass1"

	if err := CreateRootAccountIfNeed(); err != nil {
		t.Fatalf("CreateRootAccountIfNeed() error = %v", err)
	}

	var root User
	if err := DB.Where("username = ?", "root").First(&root).Error; err != nil {
		t.Fatalf("find root user: %v", err)
	}
	if !common.ValidatePasswordAndHash(config.InitialRootPassword, root.Password) {
		t.Fatal("root password hash does not match INITIAL_ROOT_PASSWORD")
	}
}

func TestCreateRootAccountIfNeedLeavesExistingDatabaseUnchanged(t *testing.T) {
	withRootAccountTestDatabase(t)
	existing := User{
		Username: "existing",
		Password: "existing-hash",
		Role:     RoleRootUser,
		Status:   UserStatusEnabled,
	}
	if err := DB.Create(&existing).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	config.InitialRootPassword = ""

	if err := CreateRootAccountIfNeed(); err != nil {
		t.Fatalf("existing database must not require bootstrap password: %v", err)
	}

	var count int64
	if err := DB.Model(&User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("user count = %d, want 1", count)
	}
}

func TestCreateRootAccountIfNeedToleratesConcurrentBootstrap(t *testing.T) {
	originalDB := DB
	originalPassword := config.InitialRootPassword
	originalToken := config.InitialRootToken
	originalAccessToken := config.InitialRootAccessToken
	t.Cleanup(func() {
		DB = originalDB
		config.InitialRootPassword = originalPassword
		config.InitialRootToken = originalToken
		config.InitialRootAccessToken = originalAccessToken
	})

	database, err := gorm.Open(sqlite.Open("file:root-bootstrap?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open shared in-memory database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get database connection: %v", err)
	}
	sqlDB.SetMaxOpenConns(2)
	if err := database.AutoMigrate(&User{}, &Token{}); err != nil {
		t.Fatalf("migrate shared database: %v", err)
	}
	DB = database
	config.InitialRootPassword = "strong-pass1"
	config.InitialRootToken = ""
	config.InitialRootAccessToken = ""

	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errorsCh <- CreateRootAccountIfNeed()
		}()
	}
	close(start)
	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("concurrent bootstrap returned error: %v", err)
		}
	}

	var count int64
	if err := DB.Model(&User{}).Where("username = ?", "root").Count(&count).Error; err != nil {
		t.Fatalf("count root users: %v", err)
	}
	if count != 1 {
		t.Fatalf("root user count = %d, want 1", count)
	}
}

func withRootAccountTestDatabase(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalPassword := config.InitialRootPassword
	originalToken := config.InitialRootToken
	originalAccessToken := config.InitialRootAccessToken
	t.Cleanup(func() {
		DB = originalDB
		config.InitialRootPassword = originalPassword
		config.InitialRootToken = originalToken
		config.InitialRootAccessToken = originalAccessToken
	})

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := database.AutoMigrate(&User{}, &Token{}); err != nil {
		t.Fatalf("migrate in-memory database: %v", err)
	}
	DB = database
	config.InitialRootToken = ""
	config.InitialRootAccessToken = ""
}
