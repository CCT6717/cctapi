package fallback

import (
	"testing"
	"time"

	relaymodel "github.com/songquanpeng/one-api/relay/model"
)

func TestDetectRequestCapabilitiesRecognizesJSONSchema(t *testing.T) {
	caps := DetectRequestCapabilities(&relaymodel.GeneralOpenAIRequest{
		ResponseFormat: &relaymodel.ResponseFormat{
			Type:       "json_schema",
			JsonSchema: &relaymodel.JSONSchema{Name: "result"},
		},
	})
	if !caps.JSON {
		t.Fatal("json_schema response format should require JSON capability")
	}
}

func TestDetectRequestCapabilitiesRecognizesAnthropicImagePart(t *testing.T) {
	caps := DetectRequestCapabilities(&relaymodel.GeneralOpenAIRequest{
		Messages: []relaymodel.Message{{
			Content: []any{map[string]any{"type": "image"}},
		}},
	})
	if !caps.Vision {
		t.Fatal("Anthropic image content should require vision capability")
	}
}

func TestIntegrationCapabilityFilterSelectsCatalogModelForJSON(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	jsonFalse := false
	jsonTrue := true
	depID := "free:kilo-e3b0c442"
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID: depID,
		Provider:     "kilo",
		Source:       ModelFetchKiloFree,
		Models: []FreeModelCatalogEntry{
			{ID: "kilo/text:free", SupportsJSON: &jsonFalse},
			{ID: "kilo/json:free", SupportsJSON: &jsonTrue},
		},
		SelectedModel: "kilo/text:free",
		LastAttemptAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	got := FilterByCapability([]DeploymentConfig{{
		ID: depID, RealModel: "kilo/text:free", Pool: "free", SupportsStream: true,
	}}, RequestCapabilities{JSON: true})
	if len(got) != 1 || got[0].RealModel != "kilo/json:free" || !got[0].SupportsJSON {
		t.Fatalf("JSON request selected wrong model: %#v", got)
	}
}

func TestCapabilityFilterPreservesStableAliasOutsideCatalog(t *testing.T) {
	cleanup := setupFreeProviderCatalogStoreTestDB(t)
	defer cleanup()

	now := time.Now().UTC()
	depID := deploymentID("openrouter", SafeKeyHash("stable-alias"))
	if err := saveFreeProviderCatalogSuccess(FreeProviderCatalogSnapshot{
		DeploymentID:  depID,
		Provider:      "openrouter",
		Source:        ModelFetchOpenRouterFree,
		Models:        []FreeModelCatalogEntry{{ID: "provider/model:free"}},
		SelectedModel: "openrouter/free",
		LastAttemptAt: now,
		LastSuccessAt: now,
	}); err != nil {
		t.Fatalf("save catalog: %v", err)
	}

	got := FilterByCapability([]DeploymentConfig{{
		ID: depID, RealModel: "openrouter/free", Pool: "free", SupportsStream: true,
	}}, RequestCapabilities{})
	if len(got) != 1 || got[0].RealModel != "openrouter/free" {
		t.Fatalf("stable provider alias was replaced by a catalog model: %#v", got)
	}
}
