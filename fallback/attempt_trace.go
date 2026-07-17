package fallback

import (
	"sort"
	"sync"
	"time"
)

const (
	recentAttemptEventLimit = 1000
	recentAttemptChainLimit = 20
	recentAttemptStepLimit  = 50
)

var recentAttemptTrace = struct {
	sync.RWMutex
	events []AttemptEvent
}{}

// AttemptTraceStep is a safe, request-chain view of one upstream attempt.
type AttemptTraceStep struct {
	CreatedAt            time.Time      `json:"created_at"`
	Provider             string         `json:"provider"`
	DeploymentID         string         `json:"deployment_id"`
	RealModel            string         `json:"real_model"`
	Outcome              AttemptOutcome `json:"outcome"`
	StatusCode           int            `json:"status_code"`
	ErrorCategory        string         `json:"error_category"`
	DurationMs           int64          `json:"duration_ms"`
	StreamWritten        bool           `json:"stream_written"`
	PlanIndex            int            `json:"plan_index"`
	UpstreamAttemptIndex int            `json:"upstream_attempt_index"`
}

// AttemptRequestChain contains recent attempts for one request.
type AttemptRequestChain struct {
	RequestID    string             `json:"request_id"`
	VirtualModel string             `json:"virtual_model"`
	StartedAt    time.Time          `json:"started_at"`
	FinishedAt   time.Time          `json:"finished_at"`
	Steps        []AttemptTraceStep `json:"steps"`
}

func recordRecentAttempt(event AttemptEvent) {
	recentAttemptTrace.Lock()
	defer recentAttemptTrace.Unlock()

	recentAttemptTrace.events = append(recentAttemptTrace.events, event)
	if overflow := len(recentAttemptTrace.events) - recentAttemptEventLimit; overflow > 0 {
		copy(recentAttemptTrace.events, recentAttemptTrace.events[overflow:])
		recentAttemptTrace.events = recentAttemptTrace.events[:recentAttemptEventLimit]
	}
}

// SnapshotRecentAttemptChains returns copied recent request chains, newest first.
func SnapshotRecentAttemptChains(limit int) []AttemptRequestChain {
	if limit <= 0 {
		limit = recentAttemptChainLimit
	}
	if limit > recentAttemptChainLimit {
		limit = recentAttemptChainLimit
	}

	recentAttemptTrace.RLock()
	events := append([]AttemptEvent(nil), recentAttemptTrace.events...)
	recentAttemptTrace.RUnlock()

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})

	byRequest := make(map[string]*AttemptRequestChain)
	for _, event := range events {
		chain := byRequest[event.RequestID]
		if chain == nil {
			chain = &AttemptRequestChain{
				RequestID:    event.RequestID,
				VirtualModel: event.VirtualModel,
				StartedAt:    event.CreatedAt,
			}
			byRequest[event.RequestID] = chain
		}
		chain.FinishedAt = event.CreatedAt
		chain.Steps = append(chain.Steps, AttemptTraceStep{
			CreatedAt:            event.CreatedAt,
			Provider:             event.Provider,
			DeploymentID:         event.DeploymentID,
			RealModel:            event.RealModel,
			Outcome:              event.Outcome,
			StatusCode:           event.StatusCode,
			ErrorCategory:        event.ErrorCategory,
			DurationMs:           event.DurationMs,
			StreamWritten:        event.StreamWritten,
			PlanIndex:            event.PlanIndex,
			UpstreamAttemptIndex: event.UpstreamAttemptIndex,
		})
	}

	chains := make([]AttemptRequestChain, 0, len(byRequest))
	for _, chain := range byRequest {
		if len(chain.Steps) > recentAttemptStepLimit {
			chain.Steps = chain.Steps[len(chain.Steps)-recentAttemptStepLimit:]
		}
		chains = append(chains, *chain)
	}
	sort.Slice(chains, func(i, j int) bool {
		if chains[i].FinishedAt.Equal(chains[j].FinishedAt) {
			return chains[i].RequestID > chains[j].RequestID
		}
		return chains[i].FinishedAt.After(chains[j].FinishedAt)
	})
	if len(chains) > limit {
		chains = chains[:limit]
	}
	return chains
}

func resetRecentAttemptTrace() {
	recentAttemptTrace.Lock()
	defer recentAttemptTrace.Unlock()
	recentAttemptTrace.events = nil
}
