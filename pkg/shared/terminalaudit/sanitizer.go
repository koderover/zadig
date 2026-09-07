package terminalaudit

import (
	"strings"
	"sync"
)

const secretMask = "********"

type streamSanitizer struct {
	mu                 sync.Mutex
	secretsByFirstByte map[byte][]string
	pending            string
}

func NewSanitizer(secrets []string) *streamSanitizer {
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

func (s *streamSanitizer) Mask(data string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.secretsByFirstByte) == 0 {
		return data
	}
	s.pending += data
	output := s.drain(false)
	if s.pending != "" {
		s.pending = strings.Clone(s.pending)
	}
	return output
}

func (s *streamSanitizer) Flush() string {
	s.mu.Lock()
	defer s.mu.Unlock()

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
