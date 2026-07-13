package fallback

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const maxFreeProviderCatalogModels = 4096

// FreeModelCatalogEntry describes optional model-level metadata published by an
// upstream provider. Nil capability pointers mean the provider did not publish
// that capability and the registry default must be used.
type FreeModelCatalogEntry struct {
	ID             string `json:"id"`
	SupportsStream *bool  `json:"supports_stream,omitempty"`
	SupportsTools  *bool  `json:"supports_tools,omitempty"`
	SupportsJSON   *bool  `json:"supports_json,omitempty"`
	SupportsVision *bool  `json:"supports_vision,omitempty"`
	ContextLength  *int   `json:"context_length,omitempty"`
}

// FreeProviderCatalogCandidate is a complete upstream response before it is
// accepted as the current runtime catalog.
type FreeProviderCatalogCandidate struct {
	Source string                  `json:"source"`
	Models []FreeModelCatalogEntry `json:"models"`
}

func validateFreeProviderCatalog(candidate FreeProviderCatalogCandidate) (FreeProviderCatalogCandidate, error) {
	if len(candidate.Models) == 0 {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("catalog returned no models")
	}
	if len(candidate.Models) > maxFreeProviderCatalogModels {
		return FreeProviderCatalogCandidate{}, fmt.Errorf("catalog returned too many models: %d", len(candidate.Models))
	}

	validated := FreeProviderCatalogCandidate{
		Source: strings.TrimSpace(candidate.Source),
		Models: make([]FreeModelCatalogEntry, 0, len(candidate.Models)),
	}
	seen := make(map[string]struct{}, len(candidate.Models))
	for _, entry := range candidate.Models {
		var err error
		entry.ID, err = normalizeFreeProviderCatalogModelID(entry.ID)
		if err != nil {
			return FreeProviderCatalogCandidate{}, err
		}
		if _, ok := seen[entry.ID]; ok {
			return FreeProviderCatalogCandidate{}, fmt.Errorf("catalog contains duplicate model id %q", entry.ID)
		}
		seen[entry.ID] = struct{}{}
		validated.Models = append(validated.Models, cloneFreeModelCatalogEntry(entry))
	}

	sort.Slice(validated.Models, func(i, j int) bool {
		return validated.Models[i].ID < validated.Models[j].ID
	})
	return validated, nil
}

func normalizeFreeProviderCatalogModelID(modelID string) (string, error) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return "", fmt.Errorf("catalog contains empty model id")
	}
	if strings.IndexFunc(modelID, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("model id %q contains a control character", modelID)
	}
	if strings.Contains(modelID, ",") {
		return "", fmt.Errorf("model id %q contains a comma delimiter", modelID)
	}
	return modelID, nil
}

func cloneFreeModelCatalogEntry(src FreeModelCatalogEntry) FreeModelCatalogEntry {
	dst := src
	dst.SupportsStream = cloneBoolPtr(src.SupportsStream)
	dst.SupportsTools = cloneBoolPtr(src.SupportsTools)
	dst.SupportsJSON = cloneBoolPtr(src.SupportsJSON)
	dst.SupportsVision = cloneBoolPtr(src.SupportsVision)
	dst.ContextLength = cloneIntPtr(src.ContextLength)
	return dst
}

func applyFreeModelCapabilities(dep DeploymentConfig, entry FreeModelCatalogEntry) DeploymentConfig {
	dep.RealModel = entry.ID
	if entry.SupportsStream != nil {
		dep.SupportsStream = *entry.SupportsStream
	}
	if entry.SupportsTools != nil {
		dep.SupportsTools = *entry.SupportsTools
	}
	if entry.SupportsJSON != nil {
		dep.SupportsJSON = *entry.SupportsJSON
	}
	if entry.SupportsVision != nil {
		dep.SupportsVision = *entry.SupportsVision
	}
	if entry.ContextLength != nil && *entry.ContextLength > 0 {
		dep.ContextLength = *entry.ContextLength
	}
	return dep
}

func freeProviderCatalogModelIDs(models []FreeModelCatalogEntry) []string {
	ids := make([]string, 0, len(models))
	for _, entry := range models {
		if strings.TrimSpace(entry.ID) != "" {
			ids = append(ids, entry.ID)
		}
	}
	return ids
}

func findFreeModelCatalogEntry(models []FreeModelCatalogEntry, modelID string) (FreeModelCatalogEntry, bool) {
	modelID = strings.TrimSpace(modelID)
	for _, entry := range models {
		if entry.ID == modelID {
			return cloneFreeModelCatalogEntry(entry), true
		}
	}
	return FreeModelCatalogEntry{}, false
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}

func boolPtr(value bool) *bool {
	return &value
}

func catalogIntPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
