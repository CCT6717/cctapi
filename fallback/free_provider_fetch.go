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

const maxFreeProviderCatalogResponseBytes = 8 << 20

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
	candidate, err := fetchProviderCatalog(providerName, key)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func fetchProviderCatalog(providerName, key string) (FreeProviderCatalogCandidate, error) {
	meta, ok := BuiltinFreeProviders[providerName]
	if !ok {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("fetchProviderCatalog: unsupported provider %q", providerName)
	}

	switch meta.ModelFetchMode {
	case ModelFetchOpenRouterFree:
		return fetchOpenRouterCatalog(key)
	case ModelFetchKiloFree:
		return fetchKiloCatalog()
	case ModelFetchOpenAIModels:
		return fetchOpenAICompatCatalog(meta.DefaultBaseURL, key)
	case ModelFetchOVHChat:
		return fetchOVHChatCatalog(meta.DefaultBaseURL, key)
	case ModelFetchStatic, "":
		models := make([]FreeModelCatalogEntry, 0, len(meta.DefaultModels))
		for _, modelID := range meta.DefaultModels {
			models = append(models, FreeModelCatalogEntry{ID: modelID})
		}
		return validateFreeProviderCatalog(FreeProviderCatalogCandidate{
			Source: ModelFetchStatic,
			Models: models,
		})
	default:
		return FreeProviderCatalogCandidate{}, fmt.Errorf("fetchProviderCatalog: unsupported fetch mode %q for provider %q", meta.ModelFetchMode, providerName)
	}
}

func fetchOpenRouterModels(key string) ([]string, error) {
	candidate, err := fetchOpenRouterCatalog(key)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func fetchOpenRouterCatalog(key string) (FreeProviderCatalogCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("openrouter /v1/models request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("openrouter /v1/models status %d", resp.StatusCode)
	}
	body, err := readFreeProviderCatalogResponseBody(resp)
	if err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("read models body: %w", err)
	}
	candidate, err := parseOpenRouterFreeCatalog(body)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	return validateFreeProviderCatalog(candidate)
}

func parseFreeModels(body []byte) ([]string, error) {
	candidate, err := parseOpenRouterFreeCatalog(body)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func parseOpenRouterFreeCatalog(body []byte) (FreeProviderCatalogCandidate, error) {
	var respData openRouterModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("parse models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	var free []FreeModelCatalogEntry
	for _, m := range respData.Data {
		if strings.HasSuffix(m.ID, ":free") {
			if _, ok := seen[m.ID]; !ok {
				seen[m.ID] = struct{}{}
				free = append(free, FreeModelCatalogEntry{ID: m.ID})
			}
		}
	}
	sort.Slice(free, func(i, j int) bool { return free[i].ID < free[j].ID })
	return FreeProviderCatalogCandidate{Source: ModelFetchOpenRouterFree, Models: free}, nil
}

type kiloModelsResponse struct {
	Data []struct {
		ID                  string   `json:"id"`
		IsFree              bool     `json:"isFree"`
		ContextLength       int      `json:"context_length"`
		SupportedParameters []string `json:"supported_parameters"`
		Architecture        struct {
			InputModalities []string `json:"input_modalities"`
		} `json:"architecture"`
		TopProvider struct {
			ContextLength int `json:"context_length"`
		} `json:"top_provider"`
	} `json:"data"`
}

func fetchKiloModels() ([]string, error) {
	candidate, err := fetchKiloCatalog()
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func fetchKiloCatalog() (FreeProviderCatalogCandidate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.kilo.ai/api/gateway/models", nil)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("kilo /models request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("kilo /models status %d", resp.StatusCode)
	}
	body, err := readFreeProviderCatalogResponseBody(resp)
	if err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("read kilo models body: %w", err)
	}
	candidate, err := parseKiloFreeCatalog(body)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	return validateFreeProviderCatalog(candidate)
}

func parseKiloFreeModels(body []byte) ([]string, error) {
	candidate, err := parseKiloFreeCatalog(body)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func parseKiloFreeCatalog(body []byte) (FreeProviderCatalogCandidate, error) {
	var respData kiloModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("parse kilo models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	models := make([]FreeModelCatalogEntry, 0, len(respData.Data))
	for _, m := range respData.Data {
		if !m.IsFree || strings.TrimSpace(m.ID) == "" {
			continue
		}
		if _, ok := seen[m.ID]; ok {
			continue
		}
		seen[m.ID] = struct{}{}
		parameters := make(map[string]struct{}, len(m.SupportedParameters))
		for _, parameter := range m.SupportedParameters {
			parameters[strings.ToLower(strings.TrimSpace(parameter))] = struct{}{}
		}
		_, supportsTools := parameters["tools"]
		if !supportsTools {
			_, supportsTools = parameters["tool_choice"]
		}
		_, supportsJSON := parameters["structured_outputs"]
		if !supportsJSON {
			_, supportsJSON = parameters["response_format"]
		}
		supportsVision := false
		for _, modality := range m.Architecture.InputModalities {
			if strings.EqualFold(strings.TrimSpace(modality), "image") {
				supportsVision = true
				break
			}
		}
		contextLength := m.ContextLength
		if contextLength <= 0 {
			contextLength = m.TopProvider.ContextLength
		}
		models = append(models, FreeModelCatalogEntry{
			ID:             m.ID,
			SupportsTools:  boolPtr(supportsTools),
			SupportsJSON:   boolPtr(supportsJSON),
			SupportsVision: boolPtr(supportsVision),
			ContextLength:  catalogIntPtr(contextLength),
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return FreeProviderCatalogCandidate{Source: ModelFetchKiloFree, Models: models}, nil
}

func fetchOpenAICompatModels(baseURL, key string) ([]string, error) {
	candidate, err := fetchOpenAICompatCatalog(baseURL, key)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func fetchOpenAICompatCatalog(baseURL, key string) (FreeProviderCatalogCandidate, error) {
	body, err := fetchOpenAICompatModelsBody(baseURL, key)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	candidate, err := parseOpenAICompatCatalog(body)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	return validateFreeProviderCatalog(candidate)
}

func fetchOVHChatCatalog(baseURL, key string) (FreeProviderCatalogCandidate, error) {
	body, err := fetchOpenAICompatModelsBody(baseURL, key)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	candidate, err := parseOVHChatCatalog(body)
	if err != nil {
		return FreeProviderCatalogCandidate{}, err
	}
	return validateFreeProviderCatalog(candidate)
}

func fetchOpenAICompatModelsBody(baseURL, key string) ([]byte, error) {
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
	body, err := readFreeProviderCatalogResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("read models body from %s: %w", modelsURL, err)
	}
	return body, nil
}

func readFreeProviderCatalogResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("catalog response body is empty")
	}
	if resp.ContentLength > maxFreeProviderCatalogResponseBytes {
		return nil, fmt.Errorf("catalog response exceeds %d bytes", maxFreeProviderCatalogResponseBytes)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFreeProviderCatalogResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxFreeProviderCatalogResponseBytes {
		return nil, fmt.Errorf("catalog response exceeds %d bytes", maxFreeProviderCatalogResponseBytes)
	}
	return body, nil
}

type openAICompatModelsResponse struct {
	Data []struct {
		ID                  string `json:"id"`
		MaxCompletionTokens int    `json:"max_completion_tokens"`
		ContextLength       int    `json:"context_length"`
	} `json:"data"`
}

func parseOpenAICompatModels(body []byte) ([]string, error) {
	candidate, err := parseOpenAICompatCatalog(body)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(candidate.Models))
	for _, entry := range candidate.Models {
		models = append(models, entry.ID)
	}
	return models, nil
}

func parseOpenAICompatCatalog(body []byte) (FreeProviderCatalogCandidate, error) {
	var respData openAICompatModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("parse models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	var models []FreeModelCatalogEntry
	for _, m := range respData.Data {
		if m.ID == "" {
			continue
		}
		if _, ok := seen[m.ID]; !ok {
			seen[m.ID] = struct{}{}
			models = append(models, FreeModelCatalogEntry{ID: m.ID})
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return FreeProviderCatalogCandidate{Source: ModelFetchOpenAIModels, Models: models}, nil
}

func parseOVHChatCatalog(body []byte) (FreeProviderCatalogCandidate, error) {
	var respData openAICompatModelsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("parse OVH models json: %w", err)
	}
	seen := make(map[string]struct{}, len(respData.Data))
	models := make([]FreeModelCatalogEntry, 0, len(respData.Data))
	for _, model := range respData.Data {
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || model.MaxCompletionTokens <= 0 {
			continue
		}
		if _, ok := seen[model.ID]; ok {
			continue
		}
		seen[model.ID] = struct{}{}
		models = append(models, FreeModelCatalogEntry{
			ID:            model.ID,
			ContextLength: catalogIntPtr(model.ContextLength),
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return FreeProviderCatalogCandidate{Source: ModelFetchOVHChat, Models: models}, nil
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
