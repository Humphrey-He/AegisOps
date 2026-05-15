package redact

import (
	"encoding/json"
	"reflect"
	"strings"
)

const Mask = "***REDACTED***"

var sensitiveTokens = []string{
	"secret",
	"password",
	"token",
	"privatekey",
	"private_key",
	"certificate",
	"cert",
	"keypem",
	"webhook",
	"botToken",
	"bot_token",
	"smtp",
	"jwt",
	"session",
}

func Struct(value any, masked bool) any {
	if !masked || value == nil {
		return value
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return value
	}
	return Value(decoded)
}

func Value(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			if IsSensitiveKey(key) {
				if item == nil || item == "" {
					result[key] = item
				} else {
					result[key] = Mask
				}
				continue
			}
			result[key] = Value(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, Value(item))
		}
		return result
	default:
		return value
	}
}

func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	for _, token := range sensitiveTokens {
		if strings.Contains(normalized, strings.ToLower(token)) {
			return true
		}
	}
	return false
}

func JSON(value any, masked bool) ([]byte, error) {
	return json.MarshalIndent(Struct(value, masked), "", "  ")
}

func CopyNonZeroString(value string, masked bool) string {
	if masked && strings.TrimSpace(value) != "" {
		return Mask
	}
	return value
}

func ShallowMap(value any, masked bool) map[string]any {
	raw := Struct(value, masked)
	if mapped, ok := raw.(map[string]any); ok {
		return mapped
	}
	result := map[string]any{}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return result
	}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		key := strings.Split(field.Tag.Get("json"), ",")[0]
		if key == "" || key == "-" {
			key = field.Name
		}
		item := rv.Field(i).Interface()
		if masked && IsSensitiveKey(key) {
			result[key] = Mask
		} else {
			result[key] = item
		}
	}
	return result
}
