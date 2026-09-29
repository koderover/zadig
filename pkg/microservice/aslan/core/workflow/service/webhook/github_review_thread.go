package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/reviewfeedback"
	"go.uber.org/zap"
)

// The current go-github version does not decode review-thread webhooks.
type githubReviewThreadEvent struct {
	Action     string `json:"action"`
	Repository struct {
		Name    string `json:"name"`
		HTMLURL string `json:"html_url"`
		Owner   struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Thread struct {
		NodeID   string `json:"node_id"`
		Comments []struct {
			ID int64 `json:"id"`
		} `json:"comments"`
	} `json:"thread"`
}

func processGitHubReviewThread(payload []byte, req *http.Request, logger *zap.SugaredLogger) error {
	var event githubReviewThreadEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}
	logger.Infow("received github review thread webhook", "action", event.Action, "repo", event.Repository.Name, "pr", event.PullRequest.Number, "thread_id", event.Thread.NodeID)
	if event.Action != "resolved" && event.Action != "unresolved" {
		logger.Infow("ignored github review thread webhook", "reason", "unsupported action")
		return nil
	}
	sourceHost := reviewfeedback.Host(event.Repository.HTMLURL)
	if sourceHost == "" || event.Repository.Owner.Login == "" || event.Repository.Name == "" || event.PullRequest.Number <= 0 {
		return fmt.Errorf("GitHub review thread webhook is missing repository/PR identity")
	}
	ids := make([]int64, 0, len(event.Thread.Comments))
	for _, comment := range event.Thread.Comments {
		if comment.ID > 0 {
			ids = append(ids, comment.ID)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("GitHub review thread webhook has no comment IDs")
	}
	if event.Thread.NodeID == "" {
		return fmt.Errorf("GitHub review thread webhook is missing node_id")
	}
	return reviewfeedback.ApplyGitHubThread(req.Context(), sourceHost, event.Repository.Owner.Login, event.Repository.Name, event.PullRequest.Number, ids, event.Thread.NodeID, logger)
}
