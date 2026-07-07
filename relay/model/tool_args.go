package model

import (
	"encoding/json"
	"strings"
)

// RepairToolArguments normalizes common tool-call argument shapes emitted by
// OpenAI-compatible upstreams while preserving invalid or schema-ambiguous data.
func RepairToolArguments(arguments string, parameterSchema any) (string, bool) {
	var parsed any
	if err := json.Unmarshal([]byte(arguments), &parsed); err != nil {
		return arguments, false
	}

	changed := false
	if wrapped, ok := parsed.(string); ok {
		var unwrapped any
		if err := json.Unmarshal([]byte(wrapped), &unwrapped); err != nil {
			return arguments, false
		}
		if _, ok := unwrapped.(map[string]any); !ok {
			return arguments, false
		}
		parsed = unwrapped
		changed = true
	}

	args, ok := parsed.(map[string]any)
	if !ok {
		return arguments, false
	}

	props := schemaProperties(parameterSchema)
	for name, value := range args {
		rawString, ok := value.(string)
		if !ok {
			continue
		}

		wantType := schemaType(props[name])
		if wantType != "array" && wantType != "object" {
			continue
		}

		trimmed := strings.TrimSpace(rawString)
		if wantType == "array" && !strings.HasPrefix(trimmed, "[") {
			continue
		}
		if wantType == "object" && !strings.HasPrefix(trimmed, "{") {
			continue
		}

		var nested any
		if err := json.Unmarshal([]byte(trimmed), &nested); err != nil {
			continue
		}
		if wantType == "array" {
			if _, ok := nested.([]any); !ok {
				continue
			}
		} else if _, ok := nested.(map[string]any); !ok {
			continue
		}

		args[name] = nested
		changed = true
	}

	if !changed {
		return arguments, false
	}

	repaired, err := json.Marshal(parsed)
	if err != nil {
		return arguments, false
	}
	return string(repaired), true
}

func schemaProperties(schema any) map[string]any {
	root := normalizeSchemaMap(schema)
	props := normalizeSchemaMap(root["properties"])
	if props == nil {
		return map[string]any{}
	}
	return props
}

func schemaType(schema any) string {
	node := normalizeSchemaMap(schema)
	if node == nil {
		return ""
	}
	value, _ := node["type"].(string)
	return value
}

func normalizeSchemaMap(schema any) map[string]any {
	if schema == nil {
		return nil
	}
	if value, ok := schema.(map[string]any); ok {
		return value
	}

	raw, err := json.Marshal(schema)
	if err != nil {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}
	return value
}

func RepairChatCompletionToolArgumentsJSON(body []byte, tools []Tool) ([]byte, bool, error) {
	toolSchemas := make(map[string]any)
	for _, tool := range tools {
		if tool.Type != "function" || tool.Function.Name == "" || tool.Function.Parameters == nil {
			continue
		}
		toolSchemas[tool.Function.Name] = tool.Function.Parameters
	}
	if len(toolSchemas) == 0 {
		return body, false, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body, false, err
	}

	changed := false
	choices, _ := payload["choices"].([]any)
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		if message == nil {
			continue
		}

		toolCalls, _ := message["tool_calls"].([]any)
		for _, rawToolCall := range toolCalls {
			toolCall, _ := rawToolCall.(map[string]any)
			function, _ := toolCall["function"].(map[string]any)
			if function == nil {
				continue
			}

			name, _ := function["name"].(string)
			schema, ok := toolSchemas[name]
			if !ok {
				continue
			}

			arguments, ok := function["arguments"].(string)
			if !ok {
				continue
			}
			repaired, didRepair := RepairToolArguments(arguments, schema)
			if !didRepair {
				continue
			}

			function["arguments"] = repaired
			changed = true
		}
	}

	if !changed {
		return body, false, nil
	}

	repairedBody, err := json.Marshal(payload)
	if err != nil {
		return body, false, err
	}
	return repairedBody, true, nil
}
