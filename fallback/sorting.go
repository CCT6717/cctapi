package fallback

import (
	"math"
	"sort"
	"strings"
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

// ——————— Strategy-aware scoring ———————

// strategyScore re-ranks deployments according to the virtual model's strategy.
// quality_first: lean on QualityTier, success rate, low latency.
// cost_first:    lean on CostTier (free/cheap), headroom, health.
// free_first:    lean on headroom, rate-limit penalty, jitter to spread keys.
//
// Jitter is a small deterministic per-deployment offset so free keys aren't
// all hit in the same order every request.
func strategyScore(dep DeploymentConfig, state *DeploymentState, strategy string) float64 {
	const jitterMax = 5.0
	jitter := deterministicJitter(dep.ID, jitterMax)

	headroom := headroomRatio(dep, state)
	successRate := successRateOf(state)
	healthScore := healthScore(dep, state)
	rlPenalty := rateLimitPenalty(dep.ID)

	switch strategy {
	case StrategyCostFirst:
		costScore := costTierScore(dep.CostTier)
		return costScore*0.35 + healthScore*0.25 + headroom*0.20 + successRate*0.10 + jitter*0.10
	case StrategyFreeFirst:
		return headroom*0.35 + healthScore*0.25 + rlPenalty*0.20 + successRate*0.10 + jitter*0.10
	case StrategyQualityFirst:
		fallthrough
	default:
		qualityScore := qualityTierScore(dep.QualityTier)
		return qualityScore*0.40 + healthScore*0.25 + successRate*0.20 + jitter*0.10 + headroom*0.05
	}
}

func qualityTierScore(tier string) float64 {
	switch normalizeTier(tier) {
	case "high":
		return 100
	case "medium":
		return 60
	default:
		return 30
	}
}

func costTierScore(tier string) float64 {
	switch normalizeTier(tier) {
	case "free":
		return 100
	case "cheap":
		return 70
	default: // paid
		return 30
	}
}

// headroomRatio returns 0..1 of how much daily token budget remains.
// Deployments with no daily limit (free/upstream-managed) get full headroom.
func headroomRatio(dep DeploymentConfig, state *DeploymentState) float64 {
	if dep.DailyLimitTokens <= 0 {
		return 1.0
	}
	if state == nil {
		return 1.0
	}
	used := float64(state.UsedTotalTokens)
	limit := float64(dep.DailyLimitTokens)
	if limit <= 0 {
		return 1.0
	}
	remaining := (limit - used) / limit
	if remaining < 0 {
		return 0
	}
	if remaining > 1 {
		return 1
	}
	return remaining
}

func successRateOf(state *DeploymentState) float64 {
	if state == nil || state.RequestCount == 0 {
		return 1.0 // no history, assume fine
	}
	return float64(state.SuccessCount) / float64(state.RequestCount)
}

// healthScore maps cooldown/exhausted/recent-error into a 0..100 health score.
func healthScore(dep DeploymentConfig, state *DeploymentState) float64 {
	if state == nil {
		return 100
	}
	now := time.Now()
	if state.ExhaustedUntil != nil && state.ExhaustedUntil.After(now) {
		return 0
	}
	if cooldownUntil, _, err := GetDeploymentCooldown(dep.ID); err == nil && cooldownUntil != nil && cooldownUntil.After(now) {
		return 0
	}
	score := 100.0
	if state.UpdatedAt.After(now.Add(-5*time.Minute)) && state.LastErrorCode != "" && state.LastErrorCode != "exhausted" {
		score -= 30
	}
	return score
}

// rateLimitPenalty returns a 0..100 score that is HIGH when the rate-limit
// penalty is LOW (so it adds positively to the free_first score).
func rateLimitPenalty(deploymentID string) float64 {
	snap := SnapshotRuntimeState(deploymentID)
	// score = 100 minus 10 per penalty point
	score := 100 - snap.RateLimitScore*10
	if score < 0 {
		return 0
	}
	return float64(score)
}

func normalizeTier(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "high":
		return "high"
	case "medium":
		return "medium"
	case "low":
		return "low"
	case "free":
		return "free"
	case "cheap":
		return "cheap"
	case "paid":
		return "paid"
	default:
		return ""
	}
}

// deterministicJitter produces a stable 0..max pseudo-random value from the
// deployment id, so the same key isn't always picked first in free_first.
// Math/rand is avoided to keep this pure and test-friendly.
func deterministicJitter(deploymentID string, max float64) float64 {
	if max <= 0 || deploymentID == "" {
		return 0
	}
	var sum uint64
	for _, b := range []byte(deploymentID) {
		sum = sum*31 + uint64(b)
	}
	return float64(sum%1000) / 1000.0 * max
}

// SortByStrategy sorts a copy of deployments by the given strategy and returns it.
func SortByStrategy(deployments []DeploymentConfig, strategy string) []DeploymentConfig {
	out := make([]DeploymentConfig, len(deployments))
	copy(out, deployments)
	today := todayString()
	sort.SliceStable(out, func(i, j int) bool {
		si, _ := GetDeploymentState(out[i].ID, today)
		sj, _ := GetDeploymentState(out[j].ID, today)
		return strategyScore(out[i], si, strategy) > strategyScore(out[j], sj, strategy)
	})
	return out
}
