package fallback

import (
	"strings"
	"testing"
)

func TestParseKiloFreeCatalogExtractsPerModelCapabilities(t *testing.T) {
	body := []byte(`{
		"data": [
			{
				"id": "text/tools:free",
				"isFree": true,
				"context_length": 131072,
				"supported_parameters": ["max_tokens", "tools", "structured_outputs"],
				"architecture": {"input_modalities": ["text"]}
			},
			{
				"id": "vision/no-tools:free",
				"isFree": true,
				"top_provider": {"context_length": 262144},
				"supported_parameters": ["temperature"],
				"architecture": {"input_modalities": ["text", "image"]}
			},
			{"id": "paid/model", "isFree": false}
		]
	}`)

	candidate, err := parseKiloFreeCatalog(body)
	if err != nil {
		t.Fatalf("parseKiloFreeCatalog error: %v", err)
	}
	if candidate.Source != ModelFetchKiloFree {
		t.Fatalf("source = %q, want %q", candidate.Source, ModelFetchKiloFree)
	}
	if len(candidate.Models) != 2 {
		t.Fatalf("model count = %d, want 2", len(candidate.Models))
	}

	models := map[string]FreeModelCatalogEntry{}
	for _, entry := range candidate.Models {
		models[entry.ID] = entry
	}
	tools := models["text/tools:free"]
	if tools.SupportsTools == nil || !*tools.SupportsTools {
		t.Fatal("tools model should advertise tools")
	}
	if tools.SupportsJSON == nil || !*tools.SupportsJSON {
		t.Fatal("tools model should advertise structured JSON")
	}
	if tools.SupportsVision == nil || *tools.SupportsVision {
		t.Fatal("text-only model should explicitly disable vision")
	}
	if tools.ContextLength == nil || *tools.ContextLength != 131072 {
		t.Fatalf("context length = %v, want 131072", tools.ContextLength)
	}

	vision := models["vision/no-tools:free"]
	if vision.SupportsVision == nil || !*vision.SupportsVision {
		t.Fatal("vision model should advertise vision")
	}
	if vision.SupportsTools == nil || *vision.SupportsTools {
		t.Fatal("model without tools parameter should explicitly disable tools")
	}
	if vision.SupportsJSON == nil || *vision.SupportsJSON {
		t.Fatal("model without structured output parameter should disable JSON")
	}
	if vision.ContextLength == nil || *vision.ContextLength != 262144 {
		t.Fatalf("top-provider context length = %v, want 262144", vision.ContextLength)
	}
}

func TestValidateFreeProviderCatalogRejectsUnsafeCandidates(t *testing.T) {
	tests := []struct {
		name      string
		candidate FreeProviderCatalogCandidate
		wantError string
	}{
		{
			name:      "empty",
			candidate: FreeProviderCatalogCandidate{Source: ModelFetchOpenAIModels},
			wantError: "no models",
		},
		{
			name: "blank id",
			candidate: FreeProviderCatalogCandidate{
				Source: ModelFetchOpenAIModels,
				Models: []FreeModelCatalogEntry{{ID: "   "}},
			},
			wantError: "empty model id",
		},
		{
			name: "control character",
			candidate: FreeProviderCatalogCandidate{
				Source: ModelFetchOpenAIModels,
				Models: []FreeModelCatalogEntry{{ID: "bad\nmodel"}},
			},
			wantError: "control character",
		},
		{
			name: "duplicate",
			candidate: FreeProviderCatalogCandidate{
				Source: ModelFetchOpenAIModels,
				Models: []FreeModelCatalogEntry{{ID: "same"}, {ID: " same "}},
			},
			wantError: "duplicate model id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateFreeProviderCatalog(tt.candidate)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestValidateFreeProviderCatalogNormalizesAndSorts(t *testing.T) {
	candidate := FreeProviderCatalogCandidate{
		Source: ModelFetchOpenAIModels,
		Models: []FreeModelCatalogEntry{
			{ID: " z-model "},
			{ID: "a-model"},
		},
	}

	validated, err := validateFreeProviderCatalog(candidate)
	if err != nil {
		t.Fatalf("validateFreeProviderCatalog error: %v", err)
	}
	if got := validated.Models[0].ID; got != "a-model" {
		t.Fatalf("first model = %q, want a-model", got)
	}
	if got := validated.Models[1].ID; got != "z-model" {
		t.Fatalf("second model = %q, want z-model", got)
	}
}

func TestParseOpenAICompatCatalogKeepsCapabilitiesUnknown(t *testing.T) {
	candidate, err := parseOpenAICompatCatalog([]byte(`{
		"data": [{"id": "model-b"}, {"id": "model-a"}]
	}`))
	if err != nil {
		t.Fatalf("parseOpenAICompatCatalog error: %v", err)
	}
	if candidate.Source != ModelFetchOpenAIModels {
		t.Fatalf("source = %q, want %q", candidate.Source, ModelFetchOpenAIModels)
	}
	if len(candidate.Models) != 2 || candidate.Models[0].ID != "model-a" {
		t.Fatalf("unexpected sorted models: %#v", candidate.Models)
	}
	entry := candidate.Models[0]
	if entry.SupportsStream != nil || entry.SupportsTools != nil || entry.SupportsJSON != nil || entry.SupportsVision != nil {
		t.Fatalf("generic OpenAI metadata should stay unknown: %#v", entry)
	}
}

func TestFetchProviderCatalogStaticModeReturnsRegistryModels(t *testing.T) {
	candidate, err := fetchProviderCatalog("groq", "unused")
	if err != nil {
		t.Fatalf("fetchProviderCatalog error: %v", err)
	}
	if candidate.Source != ModelFetchStatic || len(candidate.Models) == 0 {
		t.Fatalf("unexpected static candidate: %#v", candidate)
	}
	if candidate.Models[0].ID == "" {
		t.Fatal("static catalog contains empty model id")
	}
}
