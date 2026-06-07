package ctxkey

const (
	// FallbackEnabled indicates that the request is using fallback mode
	FallbackEnabled = "fallback_enabled"

	// FallbackVirtualModel is the virtual model name being used
	FallbackVirtualModel = "fallback_virtual_model"

	// FallbackDeploymentID is the ID of the deployment being used
	FallbackDeploymentID = "fallback_deployment_id"

	// FallbackRealModel is the real model name that will be sent to upstream
	FallbackRealModel = "fallback_real_model"

	// FallbackChannelID is the channel ID that will be used
	FallbackChannelID = "fallback_channel_id"

	// FallbackDeploymentIndex is the index of current deployment being tried
	FallbackDeploymentIndex = "fallback_deployment_index"

	// FallbackAttemptCount is the attempt count (1-based)
	FallbackAttemptCount = "fallback_attempt_count"
)
