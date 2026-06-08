package errors

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	urlCandidatePattern        = regexp.MustCompile(`https?://[^\s",]+`)
	authorizationHeaderPattern = regexp.MustCompile(`(?i)(authorization[:=]\s*(?:basic|bearer)\s+)[^,\s"]+`)
	sensitiveQueryKeys         = map[string]struct{}{
		"username":             {},
		"user_name":            {},
		"password":             {},
		"passwd":               {},
		"pwd":                  {},
		"token":                {},
		"access_token":         {},
		"refresh_token":        {},
		"access_key":           {},
		"access_key_id":        {},
		"access_key_secret":    {},
		"secret":               {},
		"client_secret":        {},
		"private_access_token": {},
	}
)

func sanitizeSensitiveInfo(text string) string {
	if text == "" {
		return text
	}

	text = sanitizeURLsInText(text)
	text = authorizationHeaderPattern.ReplaceAllString(text, `${1}***`)

	return text
}

func sanitizeURLsInText(text string) string {
	return urlCandidatePattern.ReplaceAllStringFunc(text, sanitizeURLString)
}

func sanitizeURLString(raw string) string {
	raw = sanitizeURLUserInfo(raw)
	raw = sanitizeURLQuery(raw)
	return raw
}

func sanitizeURLUserInfo(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}

	schemeIdx := strings.Index(raw, "://")
	if schemeIdx < 0 {
		return raw
	}

	userInfoStart := schemeIdx + 3
	atOffset := strings.Index(raw[userInfoStart:], "@")
	if atOffset < 0 {
		return raw
	}

	replacement := "***"
	if _, hasPassword := parsed.User.Password(); hasPassword {
		replacement = "***:***"
	}

	userInfoEnd := userInfoStart + atOffset
	return raw[:userInfoStart] + replacement + raw[userInfoEnd:]
}

func sanitizeURLQuery(raw string) string {
	queryStart := strings.Index(raw, "?")
	if queryStart < 0 {
		return raw
	}

	queryEnd := len(raw)
	if fragmentStart := strings.Index(raw[queryStart+1:], "#"); fragmentStart >= 0 {
		queryEnd = queryStart + 1 + fragmentStart
	}

	queryParts := strings.Split(raw[queryStart+1:queryEnd], "&")
	for i, part := range queryParts {
		key, _, found := strings.Cut(part, "=")
		if _, ok := sensitiveQueryKeys[strings.ToLower(key)]; !ok {
			continue
		}
		if found {
			queryParts[i] = key + "=***"
		} else {
			queryParts[i] = key
		}
	}

	return raw[:queryStart+1] + strings.Join(queryParts, "&") + raw[queryEnd:]
}
