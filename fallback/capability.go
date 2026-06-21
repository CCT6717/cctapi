package fallback

import (
	"strings"

	"github.com/songquanpeng/one-api/relay/model"
)

// RequestCapabilities describes what a request needs from a deployment.
type RequestCapabilities struct {
	Vision    bool
	Stream    bool
	Tools     bool
	JSON      bool
	MaxTokens int // estimated total tokens (input + expected output)
}

// DetectRequestCapabilities inspects an OpenAI-compatible request body and
// reports which capabilities the chosen deployment must support.
func DetectRequestCapabilities(req *model.GeneralOpenAIRequest) RequestCapabilities {
	caps := RequestCapabilities{}

	if req == nil {
		return caps
	}

	caps.Stream = req.Stream

	// vision: any message content part of type image_url
	for _, msg := range req.Messages {
		if hasImageContent(msg.Content) {
			caps.Vision = true
			break
		}
	}

	// tools
	if len(req.Tools) > 0 {
		caps.Tools = true
	}

	// response_format json
	if req.ResponseFormat != nil {
		if strings.EqualFold(req.ResponseFormat.Type, "json_object") {
			caps.JSON = true
		}
	}

	// estimated tokens
	if req.MaxTokens > 0 {
		caps.MaxTokens += req.MaxTokens
	}
	if req.MaxCompletionTokens != nil && *req.MaxCompletionTokens > 0 {
		caps.MaxTokens += *req.MaxCompletionTokens
	}

	return caps
}

// hasImageContent returns true if message content contains an image_url part.
func hasImageContent(content any) bool {
	parts, ok := content.([]any)
	if !ok {
		return false
	}
	for _, part := range parts {
		m, ok := part.(map[string]any)
		if !ok {
			continue
		}
		if typ, ok := m["type"].(string); ok && strings.EqualFold(typ, "image_url") {
			return true
		}
	}
	return false
}

// DeploymentSupports returns true if the deployment can serve a request with
// the given capability requirements. A zero-value capability means "not required".
func DeploymentSupports(dep DeploymentConfig, caps RequestCapabilities) bool {
	if caps.Vision && !dep.SupportsVision {
		return false
	}
	if caps.Stream && !dep.SupportsStream {
		return false
	}
	if caps.Tools && !dep.SupportsTools {
		return false
	}
	if caps.JSON && !dep.SupportsJSON {
		return false
	}
	// context length: only filter when both are positive and request exceeds it
	if dep.ContextLength > 0 && caps.MaxTokens > 0 && caps.MaxTokens > dep.ContextLength {
		return false
	}
	return true
}

// FilterByCapability returns only deployments that support the request's needs.
func FilterByCapability(deployments []DeploymentConfig, caps RequestCapabilities) []DeploymentConfig {
	out := make([]DeploymentConfig, 0, len(deployments))
	for _, dep := range deployments {
		if DeploymentSupports(dep, caps) {
			out = append(out, dep)
		}
	}
	return out
}
