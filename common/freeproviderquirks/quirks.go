package freeproviderquirks

import (
	"encoding/hex"
	"strconv"
	"strings"
)

const AutoChannelPrefix = "[CCT Auto] "

type Quirks struct {
	ForceParallelToolCalls *bool  `json:"force_parallel_tool_calls,omitempty"`
	DefaultUserAgent       string `json:"default_user_agent,omitempty"`
	DisableStream          bool   `json:"disable_stream,omitempty"`
	MaxOutputTokens        int    `json:"max_output_tokens,omitempty"`
	DropStop               bool   `json:"drop_stop,omitempty"`
}

var forceParallelToolCallsFalse = false

var byProvider = map[string]Quirks{
	"nvidia": {
		ForceParallelToolCalls: &forceParallelToolCallsFalse,
	},
	"routeway": {
		DefaultUserAgent: "cctapi-free-pool/1.0",
	},
	"aihorde": {
		DisableStream:   true,
		MaxOutputTokens: 1024,
		DropStop:        true,
	},
}

func ForProvider(provider string) (*Quirks, bool) {
	quirks, ok := byProvider[strings.ToLower(strings.TrimSpace(provider))]
	if !ok {
		return nil, false
	}
	return Clone(&quirks), true
}

func FromAutoChannelName(channelName string) (string, *Quirks, bool) {
	if !strings.HasPrefix(channelName, AutoChannelPrefix) {
		return "", nil, false
	}
	rest := strings.TrimPrefix(channelName, AutoChannelPrefix)
	for providerName := range byProvider {
		prefix := providerName + "-"
		if !strings.HasPrefix(rest, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(rest, prefix)
		if !isAutoDeploymentSuffix(suffix) {
			return "", nil, false
		}
		quirks, _ := ForProvider(providerName)
		return providerName, quirks, true
	}
	return "", nil, false
}

func Clone(src *Quirks) *Quirks {
	if src == nil {
		return nil
	}
	dst := *src
	if src.ForceParallelToolCalls != nil {
		value := *src.ForceParallelToolCalls
		dst.ForceParallelToolCalls = &value
	}
	return &dst
}

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
