package common

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func RequestFingerprint(conversation string, req models.RunRequest) string {
	var cardOperation []any
	if c := req.UIContinuation; c != nil {
		cardOperation = []any{c.CardID, c.SubmissionID, c.Hash, c.Retry}
	}
	encoded, _ := json.Marshal(struct {
		Conversation  string
		Regenerate    string              `json:",omitempty"`
		Retry         string              `json:",omitempty"`
		CardReply     *models.UICardReply `json:",omitempty"`
		CardOperation []any               `json:",omitempty"`
		Request       models.RunRequest
	}{
		Conversation:  conversation,
		CardReply:     req.CardReply,
		CardOperation: cardOperation,
		Request:       req,
		Retry:         req.RetryFrom,
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
