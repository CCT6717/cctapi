package fallback

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/songquanpeng/one-api/relay/adaptor/mistral"
	"github.com/songquanpeng/one-api/relay/adaptor/novita"
	"github.com/songquanpeng/one-api/relay/adaptor/siliconflow"
	"github.com/songquanpeng/one-api/relay/adaptor/togetherai"
	"github.com/songquanpeng/one-api/relay/adaptor/zhipu"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

type FreeProviderMeta struct {
	ChannelType    int
	DefaultBaseURL string
	DefaultModels  []string
	DefaultRPM     int
	DefaultRPD     int
	DefaultTPM     int
	DefaultTPD     int
	ContextLength  int
	SupportsVision bool
	SupportsStream bool
	SupportsTools  bool
	SupportsJSON   bool
}

var BuiltinFreeProviders = map[string]FreeProviderMeta{
	"openrouter": {
		ChannelType:    channeltype.OpenRouter,
		DefaultBaseURL: "https://openrouter.ai/api",
		DefaultModels:  []string{"openrouter/free"},
		DefaultRPM:     20,
		ContextLength:  128000,
		SupportsStream: true,
	},
	"groq": {
		ChannelType:    channeltype.Groq,
		DefaultBaseURL: "https://api.groq.com/openai",
		DefaultModels:  []string{"mixtral-8x7b-32768", "llama-3.3-70b-versatile", "llama-3.1-8b-instant"},
		DefaultRPM:     30,
		DefaultTPM:     6000,
		ContextLength:  32768,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
	},
	"kilo": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.kilo.ai/api/gateway/v1",
		DefaultModels:  []string{}, // 由 fetchModels 动态拉取
		DefaultRPM:     10,
		ContextLength:  256000,
		SupportsStream: true,
	},
	"pollinations": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://text.pollinations.ai/openai/v1",
		DefaultModels:  []string{"openai-fast"}, // /v1/models 坏了，用静态列表
		DefaultRPM:     5,                       // keyless 匿名，限流严格
		ContextLength:  131072,
		SupportsStream: true,
	},
	"ovh": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1",
		DefaultModels: []string{
			// 仅 chat 模型（/v1/models 还返回 embeddings/image/audio，需过滤）
			"Llama-3.1-8B-Instruct",
			"Meta-Llama-3_3-70B-Instruct",
			"Mistral-7B-Instruct-v0.3",
			"Mistral-Nemo-Instruct-2407",
			"Mistral-Small-3.2-24B-Instruct-2506",
			"Qwen2.5-VL-72B-Instruct",
			"Qwen3-32B",
			"Qwen3-Coder-30B-A3B-Instruct",
			"Qwen3.5-397B-A17B",
			"Qwen3.5-9B",
			"Qwen3.6-27B",
			"Qwen3Guard-Gen-0.6B",
			"Qwen3Guard-Gen-8B",
			"gpt-oss-120b",
			"gpt-oss-20b",
		},
		DefaultRPM:     2, // keyless 匿名，2 req/min per IP per model
		ContextLength:  262144,
		SupportsStream: true,
	},
	// ── Phase 2: 启用的供应商 ──
	"siliconflow": {
		ChannelType:    channeltype.SiliconFlow,
		DefaultBaseURL: "https://api.siliconflow.cn",
		DefaultModels:  siliconflow.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"zhipu": {
		ChannelType:    channeltype.Zhipu,
		DefaultBaseURL: "https://open.bigmodel.cn",
		DefaultModels:  zhipu.ModelList,
		DefaultRPM:     5,
		ContextLength:  128000,
		SupportsStream: true,
	},
	// ── Phase 2: 预置但禁用的供应商 ──
	"mistral": {
		ChannelType:    channeltype.Mistral,
		DefaultBaseURL: "https://api.mistral.ai",
		DefaultModels:  mistral.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"togetherai": {
		ChannelType:    channeltype.TogetherAI,
		DefaultBaseURL: "https://api.together.xyz",
		DefaultModels:  togetherai.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"novita": {
		ChannelType:    channeltype.Novita,
		DefaultBaseURL: "https://api.novita.ai/v3/openai",
		DefaultModels:  novita.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"cloudflare": {
		ChannelType:    channeltype.Cloudflare,
		DefaultBaseURL: "https://api.cloudflare.com",
		DefaultModels:  []string{}, // 需特殊认证（account_id:token）
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"cerebras": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.cerebras.ai/v1", // ponytail: 占位，启用后需验证
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"sambanova": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.sambanova.ai/v1", // ponytail: 占位
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"github": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://models.inference.ai.azure.com", // GitHub Models endpoint
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"chutes": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.chutes.ai/v1", // ponytail: 占位
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"fireworks": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.fireworks.ai/inference/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"nebius": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.studio.nebius.ai/v1", // ponytail: 占位
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
	"lambdalabs": {
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.lambdalabs.com/v1", // ponytail: 占位
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
	},
}

const autoChannelPrefix = "[CCT Auto] "

func channelName(provider string, keyHash string) string {
	return autoChannelPrefix + provider + "-" + keyHash
}

func deploymentID(provider string, keyHash string) string {
	return "free:" + provider + "-" + keyHash
}

// SafeKeyHash returns a short 8-char hex hash of the API key for use in
// auto-generated channel names and deployment IDs. Uses first 4 bytes of
// SHA256 — enough to distinguish keys without exposing the full key.
func SafeKeyHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:4])
}

// knownFreeProviders lists providers that SyncFreePool can auto-manage.
// Used to distinguish auto-generated IDs from user-created ones.
var knownFreeProviders = map[string]struct{}{
	"openrouter":   {},
	"groq":         {},
	"kilo":         {},
	"pollinations": {},
	"ovh":          {},
	// Phase 2
	"siliconflow": {},
	"zhipu":       {},
	"mistral":     {},
	"togetherai":  {},
	"novita":      {},
	"cloudflare":  {},
	"cerebras":    {},
	"sambanova":   {},
	"github":      {},
	"chutes":      {},
	"fireworks":   {},
	"nebius":      {},
	"lambdalabs":  {},
}

// isAutoDeploymentSuffix validates the suffix portion of an auto-generated
// channel name or deployment ID. Accepts both the old integer index format
// (e.g. "0", "1") and the new 8-char hex hash format (e.g. "a1b2c3d4").
func isAutoDeploymentSuffix(suffix string) bool {
	// Old format: integer index
	if _, err := strconv.Atoi(suffix); err == nil {
		return true
	}
	// New format: 8-char hex hash
	if len(suffix) == 8 {
		_, err := hex.DecodeString(suffix)
		return err == nil
	}
	return false
}

// IsAutoDeploymentID returns true if the deployment ID was auto-generated by SyncFreePool.
// Strictly matches "free:{known_provider}-{suffix}" pattern where suffix is either
// an integer index (old format) or an 8-char hex hash (new format).
// User-created "free:*" IDs are excluded as they don't match known providers.
func IsAutoDeploymentID(id string) bool {
	for prov := range knownFreeProviders {
		prefix := "free:" + prov + "-"
		if strings.HasPrefix(id, prefix) {
			suffix := strings.TrimPrefix(id, prefix)
			return isAutoDeploymentSuffix(suffix)
		}
	}
	return false
}

// IsAutoChannelName returns true if the channel name was auto-generated by SyncFreePool.
// Strictly matches "[CCT Auto] {known_provider}-{suffix}" pattern.
func IsAutoChannelName(name string) bool {
	for prov := range knownFreeProviders {
		prefix := autoChannelPrefix + prov + "-"
		if strings.HasPrefix(name, prefix) {
			suffix := strings.TrimPrefix(name, prefix)
			return isAutoDeploymentSuffix(suffix)
		}
	}
	return false
}

func ValidateFreeProviderName(name string) error {
	if _, ok := BuiltinFreeProviders[name]; !ok {
		known := make([]string, 0, len(BuiltinFreeProviders))
		for k := range BuiltinFreeProviders {
			known = append(known, k)
		}
		return fmt.Errorf("unknown free provider %q, known: %s", name, strings.Join(known, ", "))
	}
	return nil
}

// ApplyLimitsOverride applies limits_override fields on top of existing int values.
// Each non-nil pointer in override replaces the corresponding value.
// Zero means unlimited (valid). Negative values should be rejected by ValidateFreeProviderLimits
// before reaching this function.
func ApplyLimitsOverride(rpm, rpd, tpm, tpd int, override *FreeProviderLimits) (int, int, int, int) {
	if override == nil {
		return rpm, rpd, tpm, tpd
	}
	if override.RPMLimit != nil {
		rpm = *override.RPMLimit
	}
	if override.RPDLimit != nil {
		rpd = *override.RPDLimit
	}
	if override.TPMLimit != nil {
		tpm = *override.TPMLimit
	}
	if override.TPDLimit != nil {
		tpd = *override.TPDLimit
	}
	return rpm, rpd, tpm, tpd
}

// ValidateFreeProviderLimits checks that all limits_override values are >= 0.
func ValidateFreeProviderLimits(limits *FreeProviderLimits) error {
	if limits == nil {
		return nil
	}
	if limits.RPMLimit != nil && *limits.RPMLimit < 0 {
		return fmt.Errorf("rpm_limit must be >= 0, got %d", *limits.RPMLimit)
	}
	if limits.RPDLimit != nil && *limits.RPDLimit < 0 {
		return fmt.Errorf("rpd_limit must be >= 0, got %d", *limits.RPDLimit)
	}
	if limits.TPMLimit != nil && *limits.TPMLimit < 0 {
		return fmt.Errorf("tpm_limit must be >= 0, got %d", *limits.TPMLimit)
	}
	if limits.TPDLimit != nil && *limits.TPDLimit < 0 {
		return fmt.Errorf("tpd_limit must be >= 0, got %d", *limits.TPDLimit)
	}
	return nil
}
