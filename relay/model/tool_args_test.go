package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepairToolArgumentsUnwrapsWholeObjectString(t *testing.T) {
	got, changed := RepairToolArguments(`"{\"query\":\"hi\"}"`, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
		},
	})
	if !changed || got != `{"query":"hi"}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsRepairsNestedArrayBySchema(t *testing.T) {
	got, changed := RepairToolArguments(`{"steps":"[{\"step\":\"ship\"}]"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{"type": "array"},
		},
	})
	if !changed || got != `{"steps":[{"step":"ship"}]}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsRepairsNestedObjectBySchema(t *testing.T) {
	got, changed := RepairToolArguments(`{"patch":"{\"file\":\"app.go\"}"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{"type": "object"},
		},
	})
	if !changed || got != `{"patch":{"file":"app.go"}}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsPreservesStringSchema(t *testing.T) {
	got, changed := RepairToolArguments(`{"payload":"{\"keep\":\"string\"}"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"payload": map[string]any{"type": "string"},
		},
	})
	if changed || got != `{"payload":"{\"keep\":\"string\"}"}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairToolArgumentsPreservesInvalidNestedJSON(t *testing.T) {
	got, changed := RepairToolArguments(`{"steps":"[{bad json]"}`, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{"type": "array"},
		},
	})
	if changed || got != `{"steps":"[{bad json]"}` {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}

func TestRepairChatCompletionToolArgumentsJSONRepairsKnownToolOnly(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","object":"chat.completion","custom":"keep","choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"update_plan","arguments":"{\"steps\":\"[{\\\"step\\\":\\\"ship\\\"}]\"}"}},{"id":"call_2","type":"function","function":{"name":"unknown","arguments":"{\"steps\":\"[{\\\"step\\\":\\\"skip\\\"}]\"}"}}]}}]}`)
	tools := []Tool{{
		Type: "function",
		Function: Function{
			Name: "update_plan",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"steps": map[string]any{"type": "array"},
				},
			},
		},
	}}

	repaired, changed, err := RepairChatCompletionToolArgumentsJSON(body, tools)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v body=%s", changed, err, repaired)
	}

	var decoded map[string]any
	if err := json.Unmarshal(repaired, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["custom"] != "keep" {
		t.Fatalf("custom field was dropped: %#v", decoded)
	}
	if !strings.Contains(string(repaired), `"arguments":"{\"steps\":[{\"step\":\"ship\"}]}"`) {
		t.Fatalf("known tool was not repaired: %s", repaired)
	}
	if !strings.Contains(string(repaired), `"name":"unknown"`) || !strings.Contains(string(repaired), `skip`) {
		t.Fatalf("unknown tool was not preserved: %s", repaired)
	}
}

func TestRepairChatCompletionToolArgumentsJSONPreservesBodyWithoutTools(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"tool_calls":[{"function":{"name":"update_plan","arguments":"{\"steps\":\"[]\"}"}}]}}]}`)
	repaired, changed, err := RepairChatCompletionToolArgumentsJSON(body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatalf("expected unchanged body, got %s", repaired)
	}
	if string(repaired) != string(body) {
		t.Fatalf("expected original body, got %s", repaired)
	}
}
