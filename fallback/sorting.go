package fallback

import (
	"math"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
)

// ——————— Smart sorting scoring ———————

// ScoreWeights controls how each factor influences the dynamic score
type ScoreWeights struct {
	BasePriorityPenalty float64 `json:"base_priority_penalty"` // penalty per priority rank above 1
	SuccessRateBonus    float64 `json:"success_rate_bonus"`    // multiplier for success rate
	ErrorRatePenalty    float64 `json:"error_rate_penalty"`    // multiplier for error rate
}

// DefaultScoreWeights returns sensible default scoring weights
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		BasePriorityPenalty: 5.0,  // each priority rank = 5 points less
		SuccessRateBonus:    30.0, // 100% success rate = +30
		ErrorRatePenalty:    50.0, // 100% error rate = -50
	}
}

// CalculateScore computes a dynamic score for a deployment based on its
// static priority and historical performance.
// Higher score = preferred earlier in the fallback chain.
func CalculateScore(dep DeploymentConfig, state *DeploymentState, weights ScoreWeights) float64 {
	if state == nil {
		// No history yet — rely on static priority
		return float64(100 - (dep.Priority-1)*int(weights.BasePriorityPenalty))
	}

	// 1. Base: derived from static priority (lower number = higher score)
	baseScore := float64(100 - (dep.Priority-1)*int(weights.BasePriorityPenalty))

	totalReqs := state.RequestCount
	if totalReqs == 0 {
		totalReqs = 1 // avoid division by zero
	}

	successRate := float64(state.SuccessCount) / float64(totalReqs)
	errorRate := float64(state.ErrorCount) / float64(totalReqs)

	// 2. Bonus for reliability
	successBonus := successRate * weights.SuccessRateBonus

	// 3. Penalty for errors
	errorPenalty := errorRate * weights.ErrorRatePenalty

	score := baseScore + successBonus - errorPenalty

	// 4. Active penalties for ongoing issues
	now := time.Now()

	// Exhausted: hard block, massive penalty
	if state.ExhaustedUntil != nil && state.ExhaustedUntil.After(now) {
		score -= 200
	}

	// Cooling down: significant penalty
	if cooldownUntil, _, err := GetDeploymentCooldown(dep.ID); err == nil && cooldownUntil != nil && cooldownUntil.After(now) {
		score -= 100
	}

	// Soft limit reached: deploy at reduced score so fallback prefers others
	if dep.DailyLimitTokens > 0 && dep.SoftLimitRatio > 0 {
		softLimit := int64(float64(dep.DailyLimitTokens) * dep.SoftLimitRatio)
		if state.UsedTotalTokens >= softLimit {
			score -= 80
		}
	}

	// Recent error (within last 5 minutes): moderate penalty
	if state.UpdatedAt.After(now.Add(-5*time.Minute)) && state.LastErrorCode != "" && state.LastErrorCode != "exhausted" {
		score -= 50
	}

	// Floor at -500 to prevent extreme negative scores from breaking sort
	return math.Max(score, -500)
}

// ——————— Smart-sorted deployment retrieval ———————

// SmartSortConfig controls the smart sorting behavior
type SmartSortConfig struct {
	Enabled bool         `json:"enabled"`
	Weights ScoreWeights `json:"weights"`
}

// DefaultSmartSortConfig returns sensible defaults
func DefaultSmartSortConfig() SmartSortConfig {
	return SmartSortConfig{
		Enabled: true,
		Weights: DefaultScoreWeights(),
	}
}

// GetDeploymentScores returns the current scores for all deployments of a virtual model
func GetDeploymentScores(virtualModel string) (map[string]float64, error) {
	deployments, err := GetDeploymentsForVirtualModel(virtualModel)
	if err != nil {
		return nil, err
	}

	configLock.RLock()
	weights := config.SmartSort.Weights
	configLock.RUnlock()

	today := todayString()
	scores := make(map[string]float64)

	for _, dep := range deployments {
		state, _ := GetDeploymentState(dep.ID, today)
		score := CalculateScore(dep, state, weights)
		scores[dep.ID] = score
	}

	if err := RecordScoreSnapshots(virtualModel, scores); err != nil {
		logger.SysError("failed to record fallback score snapshots: " + err.Error())
	}

	return scores, nil
}
