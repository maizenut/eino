package builder

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func encodeCheckpointState(state map[string]any) string {
	if len(state) == 0 {
		return ""
	}
	keys := make([]string, 0, len(state))
	for key := range state {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+encodeCheckpointValue(state[key]))
	}
	return strings.Join(parts, "\n")
}

func encodeCheckpointValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case bool:
		if typed {
			return "bool:true"
		}
		return "bool:false"
	case int:
		return "int:" + strconv.Itoa(typed)
	case int32:
		return "int:" + strconv.FormatInt(int64(typed), 10)
	case int64:
		return "int:" + strconv.FormatInt(typed, 10)
	case float32:
		return "float:" + strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case float64:
		return "float:" + strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return "string:" + typed
	case []string:
		return "strings:" + strings.Join(typed, ",")
	default:
		return "string:" + fmt.Sprint(typed)
	}
}

func decodeCheckpointState(payload string) map[string]any {
	if payload == "" {
		return map[string]any{}
	}
	state := map[string]any{}
	for _, line := range strings.Split(payload, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		state[parts[0]] = decodeCheckpointValue(parts[1])
	}
	return state
}

func decodeCheckpointValue(raw string) any {
	switch {
	case raw == "null":
		return nil
	case strings.HasPrefix(raw, "bool:"):
		return strings.TrimPrefix(raw, "bool:") == "true"
	case strings.HasPrefix(raw, "int:"):
		value := strings.TrimPrefix(raw, "int:")
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
		return value
	case strings.HasPrefix(raw, "float:"):
		value := strings.TrimPrefix(raw, "float:")
		if n, err := strconv.ParseFloat(value, 64); err == nil {
			return n
		}
		return value
	case strings.HasPrefix(raw, "strings:"):
		value := strings.TrimPrefix(raw, "strings:")
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	case strings.HasPrefix(raw, "string:"):
		return strings.TrimPrefix(raw, "string:")
	default:
		return raw
	}
}
