package terminalaudit

import (
	"github.com/koderover/zadig/v2/pkg/shared/terminalio"
	"github.com/koderover/zadig/v2/pkg/util"
)

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
