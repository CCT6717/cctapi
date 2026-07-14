package fallback

import "net/http"

// DeploymentModelAction is the request-local next step after one upstream
// model attempt fails. Provider-level accounting remains the controller's
// responsibility after this decision has been applied.
type DeploymentModelAction uint8

const (
	DeploymentModelActionComplete DeploymentModelAction = iota
	DeploymentModelActionRotate
	DeploymentModelActionSkipRemaining
)

// DeploymentModelDecision keeps HTTP rate-limit confirmation and Kilo model
// rotation semantics in one place so callers cannot apply different gates.
type DeploymentModelDecision struct {
	Action                 DeploymentModelAction
	ConfirmedHTTPRateLimit bool
	RecordModelRateLimit   bool
}

// DecideDeploymentModelAttempt classifies the next step for one failed model
// attempt. Only a confirmed HTTP 429 can rotate a Kilo catalog model.
func DecideDeploymentModelAttempt(attempt DeploymentModelAttempt, statusCode int, rateLimitCategory bool) DeploymentModelDecision {
	confirmedRateLimit := rateLimitCategory && statusCode == http.StatusTooManyRequests
	decision := DeploymentModelDecision{
		Action:                 DeploymentModelActionComplete,
		ConfirmedHTTPRateLimit: confirmedRateLimit,
	}

	if attempt.ProviderName != "kilo" || !attempt.Rotatable {
		return decision
	}
	if confirmedRateLimit {
		decision.RecordModelRateLimit = true
		if attempt.HasNextModel() {
			decision.Action = DeploymentModelActionRotate
		}
		return decision
	}
	if attempt.HasNextModel() {
		decision.Action = DeploymentModelActionSkipRemaining
	}
	return decision
}
