package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
)

const (
	releasePlanMaskedValueDisplay = "已脱敏"
	releasePlanMaskedValuePrefix  = "__masked__:"
)

func createReleasePlanLog(logItem *models.ReleasePlanLog) error {
	if logItem == nil {
		return errors.New("nil release plan log")
	}

	cloned := *logItem
	cloned.Before = sanitizeReleasePlanValue(logItem.Before)
	cloned.After = sanitizeReleasePlanValue(logItem.After)
	return mongodb.NewReleasePlanLogColl().Create(&cloned)
}

func sanitizeReleasePlanValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	genericValue, err := toReleasePlanGenericValue(value)
	if err != nil {
		return value
	}

	return sanitizeReleasePlanGenericValue("", genericValue)
}

func sanitizeReleasePlanValueForDisplay(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	genericValue, err := toReleasePlanGenericValue(value)
	if err != nil {
		if isReleasePlanMaskedStorageValue(value) {
			return releasePlanMaskedValueDisplay
		}
		return value
	}

	if hasReleasePlanRawSensitiveValue(genericValue) {
		genericValue = sanitizeReleasePlanGenericValue("", genericValue)
	}
	return sanitizeReleasePlanDisplayGenericValue(genericValue)
}

func sanitizeReleasePlanGenericValue(path string, value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			resp[key] = sanitizeReleasePlanGenericValue(joinReleasePlanMaskPath(path, key), item)
		}
		if isReleasePlanSensitiveValueNode(resp) {
			maskReleasePlanSensitiveValueNode(resp)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for idx, item := range typedValue {
			resp = append(resp, sanitizeReleasePlanGenericValue(fmt.Sprintf("%s[%d]", path, idx), item))
		}
		return resp
	default:
		return value
	}
}

func sanitizeReleasePlanDisplayGenericValue(value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			resp[key] = sanitizeReleasePlanDisplayGenericValue(item)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for _, item := range typedValue {
			resp = append(resp, sanitizeReleasePlanDisplayGenericValue(item))
		}
		return resp
	case string:
		if isReleasePlanMaskedStorageValue(typedValue) {
			return releasePlanMaskedValueDisplay
		}
		return typedValue
	default:
		return value
	}
}

func toReleasePlanGenericValue(value interface{}) (interface{}, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var resp interface{}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func maskReleasePlanValue(value interface{}) string {
	if isReleasePlanMaskedStorageValue(value) {
		if str, ok := value.(string); ok {
			return str
		}
	}

	payload, err := json.Marshal(value)
	if err != nil {
		payload = []byte(fmt.Sprintf("%v", value))
	}
	hash := sha256.Sum256(payload)
	return releasePlanMaskedValuePrefix + hex.EncodeToString(hash[:8])
}

func isReleasePlanMaskedStorageValue(value interface{}) bool {
	str, ok := value.(string)
	return ok && strings.HasPrefix(str, releasePlanMaskedValuePrefix)
}

func isReleasePlanSensitiveValueNode(value map[string]interface{}) bool {
	if value == nil {
		return false
	}
	return isReleasePlanSensitiveFlagTrue(value, "is_credential") || isReleasePlanSensitiveFlagTrue(value, "is_sensitive")
}

func hasReleasePlanRawSensitiveValue(value interface{}) bool {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		if isReleasePlanSensitiveValueNode(typedValue) {
			for _, key := range []string{"value", "choice_value"} {
				if item, exists := typedValue[key]; exists && !isReleasePlanMaskedStorageValue(item) {
					return true
				}
			}
		}
		for _, item := range typedValue {
			if hasReleasePlanRawSensitiveValue(item) {
				return true
			}
		}
	case []interface{}:
		for _, item := range typedValue {
			if hasReleasePlanRawSensitiveValue(item) {
				return true
			}
		}
	}
	return false
}

func isReleasePlanSensitiveFlagTrue(input map[string]interface{}, key string) bool {
	value, exists := input[key]
	if !exists {
		return false
	}
	flag, ok := value.(bool)
	return ok && flag
}

func maskReleasePlanSensitiveValueNode(value map[string]interface{}) {
	if value == nil {
		return
	}
	for _, key := range []string{"value", "choice_value"} {
		if item, exists := value[key]; exists {
			value[key] = maskReleasePlanValue(item)
		}
	}
}

func joinReleasePlanMaskPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
