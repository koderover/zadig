package nacos

import (
	stderrors "errors"
	"regexp"

	perrors "github.com/pkg/errors"
)

var sensitiveNacosQueryPattern = regexp.MustCompile(`(?i)((?:[?&])(?:username|user_name|password|passwd|pwd|token|access_token|refresh_token|access_key|access_key_id|access_key_secret|secret|client_secret|private_access_token)=)([^&#\s",<>()\[\]{}]*)`)

type sanitizedError struct {
	message string
	cause   error
}

func (e *sanitizedError) Error() string {
	return e.message
}

func (e *sanitizedError) Unwrap() error {
	return e.cause
}

func (e *sanitizedError) Cause() error {
	return e.cause
}

func sanitizeNacosErrorText(text string) string {
	if text == "" {
		return text
	}
	return sensitiveNacosQueryPattern.ReplaceAllString(text, `${1}***`)
}

func sanitizeNacosError(err error) error {
	if err == nil {
		return nil
	}

	sanitized := sanitizeNacosErrorText(err.Error())
	if sanitized == err.Error() {
		return err
	}

	return &sanitizedError{message: sanitized, cause: err}
}

func wrapNacosError(err error, message string) error {
	if err == nil {
		return nil
	}
	return perrors.Wrap(sanitizeNacosError(err), message)
}

func newSanitizedNacosError(message string) error {
	return stderrors.New(sanitizeNacosErrorText(message))
}
