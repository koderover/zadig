package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func RequestFingerprint(conversation string, req models.RunRequest) string {
	encoded, _ := json.Marshal(struct {
		Conversation string
		Regenerate   string `json:",omitempty"`
		Retry        string `json:",omitempty"`
		Request      models.RunRequest
	}{
		Conversation: conversation,
		Request:      req,
		Retry:        req.RetryFrom,
		Regenerate: func() string {
			if req.RegenerateFrom != nil {
				return req.RegenerateFrom.ID
			}

			return ""
		}(),
	})
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}
