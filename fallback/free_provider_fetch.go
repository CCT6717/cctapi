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
	"github.com/songquanpeng/one-api/relay/adaptor/groq"
	"github.com/songquanpeng/one-api/relay/adaptor/mistral"
	"github.com/songquanpeng/one-api/relay/adaptor/novita"
	"github.com/songquanpeng/one-api/relay/adaptor/siliconflow"
	"github.com/songquanpeng/one-api/relay/adaptor/togetherai"
	"github.com/songquanpeng/one-api/relay/adaptor/zhipu"
)

// ===== 缺口2: 模型动态获取 + 缺口4: OpenRouter 配额同步 =====
//
// 两个外部拉取操作(fetchModels / queryOpenRouterCredits)共用同一套 HTTP 模式:
// client.ImpatientHTTPClient(5s 超时) + Bearer key + json.Unmarshal。
// 定时器入口 StartFreeSync 照抄 health.go 的 StartHealthChecker 守卫模式。

// openRouterModelsResponse 是 OpenRouter GET /v1/models 的响应结构。
type openRouterModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// openRouterCreditsResponse 是 OpenRouter GET /v1/credits 的响应结构。
type openRouterCreditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

// fetchModels 拉取指定 provider 的可用模型列表。
//   - openrouter: GET /v1/models,过滤 :free 后缀,去重排序
//   - groq: 直接返静态 groq.ModelList,不网络拉
//   - kilo: GET /models,过滤 isFree:true,去重排序
//   - pollinations: /v1/models 坏了,返静态列表,不拉
//   - ovh: GET /v1/models(无需 auth),返回全部模型
//
// 失败返 error,调用方降级到静态默认。不在持有任何锁时调用。
func fetchModels(providerName, key string) ([]string, error) {
	switch providerName {
	case "groq":
		return groq.ModelList, nil
	case "openrouter":
		return fetchOpenRouterModels(key)
	case "kilo":
		return fetchKiloModels()
	case "pollinations":
		// /v1/models 返回畸形文本，用 DefaultModels 静态列表
		return BuiltinFreeProviders["pollinations"].DefaultModels, nil
	case "ovh":
		// /v1/models 返回全量（含 embeddings/image/audio），用静态列表只保留 chat 模型
		return BuiltinFreeProviders["ovh"].DefaultModels, nil
	case "siliconflow":
		return siliconflow.ModelList, nil
	case "zhipu":
		return zhipu.ModelList, nil
	case "mistral":
		return mistral.ModelList, nil
	case "togetherai":
		return togetherai.ModelList, nil
	case "novita":
		return novita.ModelList, nil
	case "cloudflare":
		// 需 account_id:token 特殊认证，暂不支持自动拉取
		return []string{}, nil
	case "cerebras", "sambanova", "github", "chutes", "fireworks", "nebius", "lambdalabs":
		// ponytail: 零基础供应商，keyless 或 placeholder，直接走通用 OpenAI 兼容拉取
		baseURL := BuiltinFreeProviders[providerName].DefaultBaseURL
		return fetchOpenAICompatModels(baseURL, key)
	default:
		return nil, fmt.Errorf("fetchModels: unsupported provider %q", providerName)
	}
}

func fetchOpenRouterModels(key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://openrouter.ai/api/v1/models", nil)
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

// parseFreeModels 解析 OpenRouter /v1/models 响应体,过滤 :free 后缀,去重排序。
// 抽成纯函数便于单测(不需打真实 HTTP)。
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

// Kilo API 响应结构
type kiloModelsResponse struct {
	Data []struct {
		ID     string `json:"id"`
		IsFree bool   `json:"isFree"`
	} `json:"data"`
}

// fetchKiloModels 调用 Kilo /models 端点,过滤 isFree:true,去重排序。
// Kilo 是 keyless 供应商，无需 API key。
func fetchKiloModels() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.kilo.ai/api/gateway/models", nil)
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

// parseKiloFreeModels 解析 Kilo /models 响应体,过滤 isFree:true,去重排序。
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

// fetchOpenAICompatModels 通用 OpenAI 兼容 /v1/models 拉取。
// 适用于 OVH 等标准 OpenAI 兼容端点。key 为空时不设 Authorization 头。
func fetchOpenAICompatModels(baseURL, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s /v1/models request: %w", baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s /v1/models status %d", baseURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read models body from %s: %w", baseURL, err)
	}
	return parseOpenAICompatModels(body)
}

// parseOpenAICompatModels 解析标准 OpenAI /v1/models 响应,去重排序。
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

// queryOpenRouterCredits 拉 OpenRouter /v1/credits,返余额(美元)= total_credits - total_usage。
func queryOpenRouterCredits(key string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://openrouter.ai/api/v1/credits", nil)
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

// parseCreditsBalance 解析 OpenRouter /v1/credits 响应体,返余额=total_credits-total_usage。
// 抽成纯函数便于单测。
func parseCreditsBalance(body []byte) (float64, error) {
	var respData openRouterCreditsResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return 0, fmt.Errorf("parse credits json: %w", err)
	}
	return respData.Data.TotalCredits - respData.Data.TotalUsage, nil
}

// syncOpenRouterCredits 是缺口4 的同步入口:遍历 openrouter,每 key 调
// queryOpenRouterCredits,算 tokens = balance * 1e6 / 7.5(OpenRouter 定价
// $7.5/1M tokens 的粗估),调 UpdateDeploymentDailyLimit 落到内存 config。
// 返更新数。不在持有 configLock 时调 HTTP —— 先拿 cfg 快照裸调 HTTP,
// 再 UpdateDeploymentDailyLimit(自己拿写锁)。
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
		// ponytail: $7.5/1M tokens 的粗估换算;真实定价按模型不同,
		// 但 free 模型实际不扣费,这里只是给 DailyLimitTokens 一个合理量级
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
