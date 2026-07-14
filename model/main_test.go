package model

import (
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
		{name: "minimum length", password: "strong-pass1"},
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
