package terminalaudit

import (
	"strings"

	"github.com/koderover/zadig/v2/pkg/shared/terminalio"
	"github.com/koderover/zadig/v2/pkg/util"
)

const secretMask = "********"

type secretSanitizer struct {
	secrets []string
}

func NewSanitizer(secrets []string) terminalio.Sanitizer {
	return &secretSanitizer{secrets: secrets}
}

func (s *secretSanitizer) Mask(data string) string {
	return util.MaskSecret(s.secrets, data)
}

type streamSanitizer struct {
	secretsByFirstByte map[byte][]string
	pending            string
}

func newStreamSanitizer(secrets []string) *streamSanitizer {
	unique := make(map[string]struct{}, len(secrets))
	for _, secret := range secrets {
		if secret != "" {
			unique[secret] = struct{}{}
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
