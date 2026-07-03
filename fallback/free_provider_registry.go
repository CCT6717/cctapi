package fallback

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/songquanpeng/one-api/relay/adaptor/geminiv2"
	"github.com/songquanpeng/one-api/relay/adaptor/groq"
	"github.com/songquanpeng/one-api/relay/adaptor/mistral"
	"github.com/songquanpeng/one-api/relay/adaptor/novita"
	"github.com/songquanpeng/one-api/relay/adaptor/siliconflow"
	"github.com/songquanpeng/one-api/relay/adaptor/togetherai"
	"github.com/songquanpeng/one-api/relay/adaptor/zhipu"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

const (
	ModelFetchStatic         = "static"
	ModelFetchOpenAIModels   = "openai_models"
	ModelFetchOpenRouterFree = "openrouter_free"
	ModelFetchKiloFree       = "kilo_free"
)

type FreeProviderMeta struct {
	ProviderID     string
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
	RequiresKey    bool
	Keyless        bool
	ModelFetchMode string
	Quirks         *FreeProviderQuirks
}

type FreeProviderQuirks struct {
	ForceParallelToolCalls *bool  `json:"force_parallel_tool_calls,omitempty"`
	DefaultUserAgent       string `json:"default_user_agent,omitempty"`
	DisableStream          bool   `json:"disable_stream,omitempty"`
	MaxOutputTokens        int    `json:"max_output_tokens,omitempty"`
	DropStop               bool   `json:"drop_stop,omitempty"`
}

var forceParallelToolCallsFalse = false

var BuiltinFreeProviders = map[string]FreeProviderMeta{
	"openrouter": {
		ProviderID:     "openrouter",
		ChannelType:    channeltype.OpenRouter,
		DefaultBaseURL: "https://openrouter.ai/api",
		DefaultModels:  []string{"openrouter/free"},
		DefaultRPM:     20,
		ContextLength:  128000,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenRouterFree,
	},
	"groq": {
		ProviderID:     "groq",
		ChannelType:    channeltype.Groq,
		DefaultBaseURL: "https://api.groq.com/openai",
		DefaultModels:  groq.ModelList,
		DefaultRPM:     30,
		DefaultTPM:     6000,
		ContextLength:  32768,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"google": {
		ProviderID:     "google",
		ChannelType:    channeltype.GeminiOpenAICompatible,
		DefaultBaseURL: "https://generativelanguage.googleapis.com/v1beta/openai",
		DefaultModels:  geminiv2.ModelList,
		DefaultRPM:     15,
		DefaultRPD:     1500,
		ContextLength:  1048576,
		SupportsVision: true,
		SupportsStream: true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"nvidia": {
		ProviderID:     "nvidia",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://integrate.api.nvidia.com/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
		Quirks: &FreeProviderQuirks{
			ForceParallelToolCalls: &forceParallelToolCallsFalse,
		},
	},
	"mistral": {
		ProviderID:     "mistral",
		ChannelType:    channeltype.Mistral,
		DefaultBaseURL: "https://api.mistral.ai",
		DefaultModels:  mistral.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"cohere": {
		ProviderID:     "cohere",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.cohere.ai/compatibility/v1",
		DefaultModels:  []string{},
		DefaultRPM:     20,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"cloudflare": {
		ProviderID:     "cloudflare",
		ChannelType:    channeltype.Cloudflare,
		DefaultBaseURL: "https://api.cloudflare.com",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"zhipu": {
		ProviderID:     "zhipu",
		ChannelType:    channeltype.Zhipu,
		DefaultBaseURL: "https://open.bigmodel.cn",
		DefaultModels:  zhipu.ModelList,
		DefaultRPM:     5,
		ContextLength:  128000,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"huggingface": {
		ProviderID:     "huggingface",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://router.huggingface.co/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"ollama": {
		ProviderID:     "ollama",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://ollama.com/v1",
		DefaultModels:  []string{},
		DefaultRPM:     5,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"kilo": {
		ProviderID:     "kilo",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.kilo.ai/api/gateway/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  256000,
		SupportsStream: true,
		Keyless:        true,
		ModelFetchMode: ModelFetchKiloFree,
	},
	"pollinations": {
		ProviderID:     "pollinations",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://text.pollinations.ai/openai/v1",
		DefaultModels:  []string{"openai-fast"},
		DefaultRPM:     5,
		ContextLength:  131072,
		SupportsStream: true,
		Keyless:        true,
		ModelFetchMode: ModelFetchStatic,
	},
	"llm7": {
		ProviderID:     "llm7",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.llm7.io/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"opencode": {
		ProviderID:     "opencode",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://opencode.ai/zen/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"ovh": {
		ProviderID:     "ovh",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1",
		DefaultModels: []string{
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
		DefaultRPM:     2,
		ContextLength:  262144,
		SupportsStream: true,
		Keyless:        true,
		ModelFetchMode: ModelFetchStatic,
	},
	"agnes": {
		ProviderID:     "agnes",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://apihub.agnes-ai.com/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"reka": {
		ProviderID:     "reka",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.reka.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsVision: true,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"siliconflow": {
		ProviderID:     "siliconflow",
		ChannelType:    channeltype.SiliconFlow,
		DefaultBaseURL: "https://api.siliconflow.cn",
		DefaultModels:  siliconflow.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"routeway": {
		ProviderID:     "routeway",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.routeway.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     5,
		DefaultRPD:     200,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
		Quirks: &FreeProviderQuirks{
			DefaultUserAgent: "cctapi-free-pool/1.0",
		},
	},
	"bazaarlink": {
		ProviderID:     "bazaarlink",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://bazaarlink.ai/api/v1",
		DefaultModels:  []string{"auto:free"},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"ainative": {
		ProviderID:     "ainative",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.ainative.studio/api/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  128000,
		SupportsStream: true,
		SupportsTools:  true,
		SupportsJSON:   true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"aihorde": {
		ProviderID:     "aihorde",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://oai.aihorde.net/v1",
		DefaultModels:  []string{},
		DefaultRPM:     2,
		ContextLength:  8192,
		SupportsStream: false,
		Keyless:        true,
		ModelFetchMode: ModelFetchOpenAIModels,
		Quirks: &FreeProviderQuirks{
			DisableStream:   true,
			MaxOutputTokens: 1024,
			DropStop:        true,
		},
	},
	"togetherai": {
		ProviderID:     "togetherai",
		ChannelType:    channeltype.TogetherAI,
		DefaultBaseURL: "https://api.together.xyz",
		DefaultModels:  togetherai.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"novita": {
		ProviderID:     "novita",
		ChannelType:    channeltype.Novita,
		DefaultBaseURL: "https://api.novita.ai/v3/openai",
		DefaultModels:  novita.ModelList,
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchStatic,
	},
	"cerebras": {
		ProviderID:     "cerebras",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.cerebras.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"sambanova": {
		ProviderID:     "sambanova",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.sambanova.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"github": {
		ProviderID:     "github",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://models.github.ai/inference",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"chutes": {
		ProviderID:     "chutes",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.chutes.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"fireworks": {
		ProviderID:     "fireworks",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.fireworks.ai/inference/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"nebius": {
		ProviderID:     "nebius",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.studio.nebius.ai/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
	},
	"lambdalabs": {
		ProviderID:     "lambdalabs",
		ChannelType:    channeltype.OpenAICompatible,
		DefaultBaseURL: "https://api.lambdalabs.com/v1",
		DefaultModels:  []string{},
		DefaultRPM:     10,
		ContextLength:  32768,
		SupportsStream: true,
		RequiresKey:    true,
		ModelFetchMode: ModelFetchOpenAIModels,
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
// auto-generated channel names and deployment IDs.
func SafeKeyHash(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:4])
}

var knownFreeProviders = initKnownFreeProviders()

func initKnownFreeProviders() map[string]struct{} {
	known := make(map[string]struct{}, len(BuiltinFreeProviders))
	for name := range BuiltinFreeProviders {
		known[name] = struct{}{}
	}
	return known
}

// isAutoDeploymentSuffix validates the suffix portion of an auto-generated
// channel name or deployment ID. Accepts both the old integer index format
// and the new 8-char hex hash format.
func isAutoDeploymentSuffix(suffix string) bool {
	if _, err := strconv.Atoi(suffix); err == nil {
		return true
	}
	if len(suffix) == 8 {
		_, err := hex.DecodeString(suffix)
		return err == nil
	}
	return false
}

// IsAutoDeploymentID returns true if the deployment ID was auto-generated by SyncFreePool.
// Strictly matches "free:{known_provider}-{suffix}" where suffix is an old index
// or an 8-char key hash. User-created "free:*" IDs are excluded.
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
// Each non-nil pointer in override replaces the corresponding value. Zero means
// unlimited. Negative values should be rejected by ValidateFreeProviderLimits first.
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
