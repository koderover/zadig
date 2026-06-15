package service

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	releasePlanRichTextBRTagPattern    = regexp.MustCompile(`(?i)<br\s*/?>`)
	releasePlanRichTextTBODYPattern    = regexp.MustCompile(`(?i)</?tbody(?:\s[^>]*)?>`)
	releasePlanRichTextTagSpacePattern = regexp.MustCompile(`>\s+<`)
)

func normalizeReleasePlanSnapshotComparableValue(path string, value interface{}) interface{} {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		resp := make(map[string]interface{}, len(typedValue))
		for key, item := range typedValue {
			resp[key] = normalizeReleasePlanSnapshotComparableValue(joinReleasePlanSnapshotComparablePath(path, key), item)
		}
		return resp
	case []interface{}:
		resp := make([]interface{}, 0, len(typedValue))
		for idx, item := range typedValue {
			resp = append(resp, normalizeReleasePlanSnapshotComparableValue(fmt.Sprintf("%s[%d]", path, idx), item))
		}
		return resp
	default:
		if isReleasePlanRichTextSnapshotComparablePath(path) {
			return normalizeReleasePlanRichTextComparableValue(value)
		}
		return value
	}
}

func joinReleasePlanSnapshotComparablePath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

func isReleasePlanRichTextSnapshotComparablePath(path string) bool {
	lowerPath := strings.ToLower(path)
	return lowerPath == "description" || strings.HasSuffix(lowerPath, ".description")
}

func normalizeReleasePlanRichTextComparableValue(value interface{}) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	normalized := normalizeReleasePlanRichTextComparableString(str)
	if isEmptyReleasePlanRichText(normalized) {
		return nil
	}
	return normalized
}

func normalizeReleasePlanRichTextComparableString(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}

	normalized = strings.ReplaceAll(normalized, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.ReplaceAll(normalized, "\u00a0", " ")
	normalized = strings.ReplaceAll(normalized, "&nbsp;", " ")
	normalized = strings.ReplaceAll(normalized, "&#160;", " ")
	normalized = releasePlanRichTextBRTagPattern.ReplaceAllString(normalized, "<br>")
	normalized = releasePlanRichTextTBODYPattern.ReplaceAllString(normalized, "")
	if !containsReleasePlanRichTextWhitespaceSensitiveTags(normalized) {
		normalized = releasePlanRichTextTagSpacePattern.ReplaceAllString(normalized, "><")
	}

	return strings.TrimSpace(normalized)
}

func containsReleasePlanRichTextWhitespaceSensitiveTags(value string) bool {
	lowerValue := strings.ToLower(value)
	return strings.Contains(lowerValue, "<pre") || strings.Contains(lowerValue, "<code")
}
