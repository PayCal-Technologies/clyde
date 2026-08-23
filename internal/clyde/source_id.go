package clyde

import "encoding/json"

func sourceIDFromJSON(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return ""
	}
	return sourceIDFromAny(value)
}

func sourceIDFromAny(value any) string {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"source_id", "sourceId", "id"} {
			if id, ok := v[key].(string); ok && id != "" {
				return id
			}
		}
		for _, nested := range v {
			if id := sourceIDFromAny(nested); id != "" {
				return id
			}
		}
	case []any:
		for _, nested := range v {
			if id := sourceIDFromAny(nested); id != "" {
				return id
			}
		}
	}
	return ""
}
