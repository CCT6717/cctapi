# Fallback Configuration Test

## File Structure Created

```
D:\project\cctapi\
├── fallback/
│   └── config.go          # Fallback configuration package
├── data/
│   └── fallback.json      # Example fallback configuration
├── test_fallback.go       # Test file to verify configuration
└── main.go                # Modified to load fallback config
```

## What Was Created

### 1. New Package: `fallback/config.go`

This package provides:
- **LoadConfig(path string) error** - Loads JSON configuration from a file
- **GetConfig() *Config** - Returns current config (read-only)
- **IsEnabled() bool** - Checks if fallback is enabled
- **IsVirtualModel(modelName string) bool** - Checks if a model is virtual
- **GetVirtualModel(modelName string) (*VirtualModelConfig, bool)** - Gets virtual model config
- **GetDeployment(id string) (*DeploymentConfig, bool)** - Gets deployment by ID
- **GetDeploymentsForVirtualModel(modelName string) ([]DeploymentConfig, error)** - Gets all deployments for a virtual model
- **ValidateConfig() error** - Validates configuration integrity

### 2. Configuration File: `data/fallback.json`

Example configuration with:
- Virtual model: `jiaobcxvcv-auto`
- 3 deployments: primary, backup_1, backup_2
- Each deployment has: channel_id, real_model, priority, limits, ratios

### 3. Modified: `main.go`

Added fallback configuration loading:
```go
// Load fallback configuration
fallbackPath := os.Getenv("FALLBACK_CONFIG_PATH")
if fallbackPath == "" {
    fallbackPath = "data/fallback.json"
}
if err := fallback.LoadConfig(fallbackPath); err != nil {
    logger.SysError("failed to load fallback config: " + err.Error())
    logger.SysError("fallback feature will be disabled")
} else {
    logger.SysLog("fallback configuration loaded successfully")
}
```

## Functions Explained

### LoadConfig(path string) error
- Loads JSON configuration from specified path
- Creates empty config if file doesn't exist (fallback disabled)
- Validates and normalizes deployment settings
- Uses mutex for thread safety

### IsEnabled() bool
- Returns true if fallback is enabled and config is loaded
- Safe to call even if config not loaded

### IsVirtualModel(modelName string) bool
- Checks if a model name matches a configured virtual model
- Returns false if fallback is disabled or model not found

### GetVirtualModel(modelName string) (*VirtualModelConfig, bool)
- Returns the virtual model configuration if exists
- Returns (nil, false) if not found or disabled

### GetDeployment(id string) (*DeploymentConfig, bool)
- Returns deployment configuration by ID
- Returns (nil, false) if not found

### GetDeploymentsForVirtualModel(modelName string) ([]DeploymentConfig, error)
- Gets all enabled deployments for a virtual model
- Returns deployments sorted by priority
- Returns error if no deployments found or virtual model disabled

### ValidateConfig() error
- Checks config integrity
- Verifies all fallback_order references valid deployments
- Ensures at least one virtual model is configured

## How to Verify Configuration Loaded

### Method 1: Run Test File

```bash
go run test_fallback.go
```

Expected output:
```
✅ Configuration loaded successfully!
✅ Fallback is enabled
✅ 'jiaobcxvcv-auto' is a virtual model
  'gpt-3.5-turbo' is not a virtual model
  'unknown-model' is not a virtual model
✅ Found virtual model: Default auto fallback model
   Fallback order: [primary backup_1 backup_2]

✅ Found 3 deployments:
   - ID: primary, ChannelID: 1, Model: gpt-4o-mini, Priority: 1
   - ID: backup_1, ChannelID: 2, Model: deepseek-chat, Priority: 2
   - ID: backup_2, ChannelID: 3, Model: qwen-plus, Priority: 3

✅ Found deployment: primary
   ChannelID: 1, RealModel: gpt-4o-mini, DailyLimit: 1000000

🎉 All tests passed!
```

### Method 2: Check Logs on Startup

When One API starts, look for:
```
fallback configuration loaded successfully
```

If not found, check for:
```
failed to load fallback config: ...
```

### Method 3: Use in Code

```go
import "github.com/songquanpeng/one-api/fallback"

// Check if model is virtual
if fallback.IsVirtualModel("jiaobcxvcv-auto") {
    deployments, err := fallback.GetDeploymentsForVirtualModel("jiaobcxvcv-auto")
    if err == nil {
        for _, dep := range deployments {
            fmt.Printf("Channel: %d, Model: %s\n", dep.ChannelID, dep.RealModel)
        }
    }
}
```

### Method 4: Validate Configuration

```go
if err := fallback.ValidateConfig(); err != nil {
    log.Printf("Configuration validation failed: %v", err)
}
```

## Configuration Format

```json
{
  "enabled": true,
  "virtual_models": {
    "model-name": {
      "enabled": true,
      "description": "Model description",
      "fallback_order": ["dep1", "dep2"]
    }
  },
  "deployments": {
    "dep1": {
      "enabled": true,
      "channel_id": 1,
      "real_model": "gpt-4",
      "priority": 1,
      "daily_limit_tokens": 1000000,
      "soft_limit_ratio": 0.9,
      "hard_limit_ratio": 0.98
    }
  }
}
```

## Environment Variable Support

You can specify configuration path via environment variable:
```bash
export FALLBACK_CONFIG_PATH=/path/to/fallback.json
./one-api
```

Or in Docker:
```bash
docker run -e FALLBACK_CONFIG_PATH=/data/fallback.json your-image
```

## Testing Checklist

- [x] New package created at `fallback/config.go`
- [x] Configuration file created at `data/fallback.json`
- [x] Configuration loading in `main.go`
- [x] All required functions implemented
- [x] Thread-safe implementation with mutex
- [x] Configuration validation function added
- [x] Test file created to verify functionality
- [x] Support for custom configuration path via environment variable
- [x] Graceful handling of missing configuration file

## Next Steps

This configuration layer is ready to be integrated with:
1. `middleware/auth.go` - Detect virtual models
2. `middleware/distributor.go` - Select fallback deployments
3. `relay/controller/text.go` - Handle deployment switching

The actual request routing logic will be added in the next phase.
