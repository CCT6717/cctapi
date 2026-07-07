package fallback

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
)

type openRouterModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type openRouterCreditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// fetchModels returns the available models for a provider. Providers choose
// their behavior through FreeProviderMeta.ModelFetchMode so adding a new
// FreeLLMAPI-style provider does not require another hard-coded switch branch.
func fetchModels(providerName, key string) ([]string, error) {
	meta, ok := BuiltinFreeProviders[providerName]
	if !ok {
		return nil, fmt.Errorf("fetchModels: unsupported provider %q", providerName)
	}

	switch meta.ModelFetchMode {
	case ModelFetchOpenRouterFree:
		return fetchOpenRouterModels(key)
	case ModelFetchKiloFree:
		return fetchKiloModels()
	case ModelFetchOpenAIModels:
		return fetchOpenAICompatModels(meta.DefaultBaseURL, key)
	case ModelFetchStatic, "":
		return append([]string{}, meta.DefaultModels...), nil
	default:
		return nil, fmt.Errorf("fetchModels: unsupported fetch mode %q for provider %q", meta.ModelFetchMode, providerName)
	}
}

func fetchOpenRouterModels(key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter /v1/models request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter /v1/models status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read models body: %w", err)
	}
	return parseFreeModels(body)
}

func parseFreeModels(body []byte) ([]string, error) {
	var respData openRouterModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("parse models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	var free []string
	for _, m := range respData.Data {
		if strings.HasSuffix(m.ID, ":free") {
			if _, ok := seen[m.ID]; !ok {
				seen[m.ID] = struct{}{}
				free = append(free, m.ID)
			}
		}
	}
	sort.Strings(free)
	return free, nil
}

type kiloModelsResponse struct {
	Data []struct {
		ID     string `json:"id"`
		IsFree bool   `json:"isFree"`
	} `json:"data"`
}

func fetchKiloModels() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.kilo.ai/api/gateway/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kilo /models request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kilo /models status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read kilo models body: %w", err)
	}
	return parseKiloFreeModels(body)
}

func parseKiloFreeModels(body []byte) ([]string, error) {
	var respData kiloModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("parse kilo models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	var free []string
	for _, m := range respData.Data {
		if m.IsFree {
			if _, ok := seen[m.ID]; !ok {
				seen[m.ID] = struct{}{}
				free = append(free, m.ID)
			}
		}
	}
	sort.Strings(free)
	return free, nil
}

func fetchOpenAICompatModels(baseURL, key string) ([]string, error) {
	modelsURL := strings.TrimRight(baseURL, "/") + "/models"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request: %w", modelsURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s status %d", modelsURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read models body from %s: %w", modelsURL, err)
	}
	return parseOpenAICompatModels(body)
}

type openAICompatModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func parseOpenAICompatModels(body []byte) ([]string, error) {
	var respData openAICompatModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("parse models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	var models []string
	for _, m := range respData.Data {
		if m.ID == "" {
			continue
		}
		if _, ok := seen[m.ID]; !ok {
			seen[m.ID] = struct{}{}
			models = append(models, m.ID)
		}
	}
	sort.Strings(models)
	return models, nil
}

func queryOpenRouterCredits(key string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/credits", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("openrouter /v1/credits request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("openrouter /v1/credits status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read credits body: %w", err)
	}
	return parseCreditsBalance(body)
}

func parseCreditsBalance(body []byte) (float64, error) {
	var respData openRouterCreditsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return 0, fmt.Errorf("parse credits json: %w", err)
	}
	return respData.Data.TotalCredits - respData.Data.TotalUsage, nil
}

func syncOpenRouterCredits(cfg *Config) int {
	if cfg == nil || cfg.FreeProviders == nil {
		return 0
	}
	fp, ok := cfg.FreeProviders["openrouter"]
	if !ok || !fp.Enabled || len(fp.Keys) == 0 {
		return 0
	}
	updated := 0
	for _, key := range fp.Keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		depID := deploymentID("openrouter", SafeKeyHash(key))
		balance, err := queryOpenRouterCredits(key)
		if err != nil {
			logger.SysWarn(fmt.Sprintf("[free-pool] openrouter credits query failed for %s: %v", depID, err))
			continue
		}
		if balance <= 0 {
			continue
		}
		tokens := int64(balance * 1_000_000 / 7.5)
		if tokens <= 0 {
			continue
		}
		UpdateDeploymentDailyLimit(depID, tokens)
		updated++
	}
	if updated > 0 {
		logger.SysLog(fmt.Sprintf("[free-pool] soft-synced %d free deployment(s) from OpenRouter /v1/credits", updated))
	}
	return updated
}
