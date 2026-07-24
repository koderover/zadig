package terminalaudit

import (
	"strings"

	"github.com/koderover/zadig/v2/pkg/shared/terminalio"
	"github.com/koderover/zadig/v2/pkg/util"
)

const secretMask = "********"

type Sanitizer = terminalio.Sanitizer

type noopSanitizer struct{}

func (n noopSanitizer) Mask(data string) string {
	return data
}

type secretSanitizer struct {
	secrets    []string
	secretEnvs []string
}

func NewSanitizer(secrets, secretEnvs []string) Sanitizer {
	if len(secrets) == 0 && len(secretEnvs) == 0 {
		return noopSanitizer{}
	}
	return &secretSanitizer{secrets: secrets, secretEnvs: secretEnvs}
}

func (s *secretSanitizer) Mask(data string) string {
	masked := data
	if len(s.secretEnvs) > 0 {
		masked = util.MaskSecretEnvs(masked, s.secretEnvs)
	}
	if len(s.secrets) > 0 {
		masked = util.MaskSecret(s.secrets, masked)
	}
	return masked
}

type streamSanitizer struct {
	secretsByFirstByte map[byte][]string
	pending            string
}

func newStreamSanitizer(secrets, secretEnvs []string) *streamSanitizer {
	unique := make(map[string]struct{}, len(secrets)+len(secretEnvs))
	for _, secret := range secrets {
		if secret != "" {
			unique[secret] = struct{}{}
		}
	}
	for _, secretEnv := range secretEnvs {
		separator := strings.IndexByte(secretEnv, '=')
		if separator >= 0 && separator < len(secretEnv)-1 {
			unique[secretEnv[separator+1:]] = struct{}{}
		}
	}

	byFirstByte := make(map[byte][]string)
	for secret := range unique {
		byFirstByte[secret[0]] = append(byFirstByte[secret[0]], secret)
	}
	return &streamSanitizer{secretsByFirstByte: byFirstByte}
}

func (s *streamSanitizer) Write(data string) string {
	if len(s.secretsByFirstByte) == 0 {
		return data
	}
	s.pending += data
	return s.drain(false)
}

func (s *streamSanitizer) Flush() string {
	if len(s.secretsByFirstByte) == 0 {
		return ""
	}
	return s.drain(true)
}

func (s *streamSanitizer) drain(final bool) string {
	var output strings.Builder
	for s.pending != "" {
		longestMatch := ""
		waitForMore := false
		for _, secret := range s.secretsByFirstByte[s.pending[0]] {
			if len(s.pending) < len(secret) && strings.HasPrefix(secret, s.pending) {
				waitForMore = true
			}
			if len(secret) > len(longestMatch) && strings.HasPrefix(s.pending, secret) {
				longestMatch = secret
			}
		}
		if waitForMore && !final {
			break
		}
		if longestMatch != "" {
			output.WriteString(secretMask)
			s.pending = s.pending[len(longestMatch):]
			continue
		}
		output.WriteByte(s.pending[0])
		s.pending = s.pending[1:]
	}
	return output.String()
}
