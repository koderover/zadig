package service

import "testing"

func TestSanitizeReleasePlanValueForDisplayMasksRawSensitiveFields(t *testing.T) {
	value := map[string]interface{}{
		"key_vals": []interface{}{
			map[string]interface{}{
				"key":           "DB_PASSWORD",
				"value":         "secret-token",
				"is_credential": true,
			},
		},
	}

	sanitized := sanitizeReleasePlanValueForDisplay(value).(map[string]interface{})
	keyVals := sanitized["key_vals"].([]interface{})
	item := keyVals[0].(map[string]interface{})
	if item["value"] != releasePlanMaskedValueDisplay {
		t.Fatalf("expected credential value to be hidden")
	}
}

func TestIsReleasePlanSensitiveValueNode(t *testing.T) {
	if isReleasePlanSensitiveValueNode(map[string]interface{}{"user_id": "alice"}) {
		t.Fatalf("plain user field should not be treated as sensitive")
	}
	if !isReleasePlanSensitiveValueNode(map[string]interface{}{"is_credential": true, "value": "secret"}) {
		t.Fatalf("credential flag should be treated as sensitive")
	}
	if !isReleasePlanSensitiveValueNode(map[string]interface{}{"is_sensitive": true, "value": "secret"}) {
		t.Fatalf("keyvault sensitive flag should be treated as sensitive")
	}
}

func TestHasReleasePlanRawSensitiveValue(t *testing.T) {
	if !hasReleasePlanRawSensitiveValue(map[string]interface{}{
		"is_credential": true,
		"value":         "secret",
	}) {
		t.Fatalf("expected raw credential value to require sanitize")
	}
	if hasReleasePlanRawSensitiveValue(map[string]interface{}{
		"is_credential": true,
		"value":         maskReleasePlanValue("secret"),
	}) {
		t.Fatalf("expected masked credential value to skip re-sanitize")
	}
}
