package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	"github.com/koderover/zadig/v2/pkg/types"
)

func submissionHash(req models.UISubmissionRequest) string {
	raw := common.JSON(struct {
		Revision int
		Action   string
		Values   map[string]any
	}{req.Revision, req.ActionID, req.Values})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (s *AgentService) cardForUser(ctx context.Context, user, auth, run, id string, cutoff time.Time) (*models.UICard, error) {
	cards, err := s.AuthorizedUICards(ctx, user, auth, run, cutoff)
	if err != nil {
		return nil, err
	}
	for _, c := range cards {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, common.ErrNotFound
}
func (s *AgentService) SubmitUICard(ctx context.Context, user, auth, runID, cardID string, req models.UISubmissionRequest, cfg *types.AIAssistantRuntime) (*models.UISubmissionResult, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.HistoryRetentionDays)
	card, err := s.cardForUser(ctx, user, auth, runID, cardID, cutoff)
	if err != nil {
		return nil, err
	}
	if req.Values == nil {
		req.Values = map[string]any{}
	}
	return s.startUICardRun(ctx, user, auth, *card, req, "", cfg, cutoff)
}

func (s *AgentService) startUICardRun(ctx context.Context, user, auth string, card models.UICard, req models.UISubmissionRequest, submissionID string, cfg *types.AIAssistantRuntime, cutoff time.Time) (*models.UISubmissionResult, error) {
	source, err := s.Store.GetRun(ctx, user, card.RunID, cutoff)
	if err != nil {
		return nil, err
	}
	u := &models.UIContinuation{
		CardID: card.ID, SourceRun: source, Revision: req.Revision, ActionID: req.ActionID, Values: req.Values,
		SubmissionID: submissionID, Retry: submissionID != "", Hash: submissionHash(req),
		Input: uiContinuationInput(card, req.ActionID, req.Values),
	}
	input := models.RunRequest{
		RequestID: req.RequestID, Context: source.Context,
		Message: uiSubmissionMessage(card, req.ActionID), UIContinuation: u,
	}
	if source.SkillExecutionID != nil {
		input.SkillExecutionID = *source.SkillExecutionID
	}
	if err := common.ValidateRequest(input); err != nil {
		return nil, fmt.Errorf("%w: %s", common.ErrInvalidRequest, err)
	}
	if _, err = s.Store.FindRequest(ctx, user, source.ConversationID, input, cutoff); err != nil {
		if !errors.Is(err, common.ErrNotFound) {
			return nil, err
		}
		if !u.Retry && (card.Status != "pending" || card.Revision != req.Revision) {
			return nil, common.ErrRequestConflict
		}
		if err := common.ValidateUIValues(card, req.ActionID, req.Values); err != nil {
			return nil, err
		}
		if card.Type == "resource_selector" {
			scopes, err := s.validateUIResources(ctx, user, auth, card, req.Values, cutoff)
			if err != nil {
				return nil, err
			}
			u.ResourceScopes = scopes
		}
	}
	if card.DraftFields && req.ActionID != "skip" {
		patch, err := onboardingCardPatch(source, req.Values)
		if err != nil {
			return nil, err
		}
		u.OnboardingPatch = &patch
	}
	if _, err := s.Start(ctx, user, source.ConversationID, auth, input, cfg); err != nil {
		return nil, err
	}
	saved, err := s.cardForUser(ctx, user, auth, card.RunID, card.ID, cutoff)
	if err != nil {
		return nil, err
	}
	if saved.Submission == nil {
		return nil, common.ErrRequestConflict
	}
	result := *saved.Submission
	result.Card = saved
	return &result, nil
}

// Keep field identity and JSON types for the model; the visible message stays human-readable.
func uiContinuationInput(card models.UICard, actionID string, values map[string]any) json.RawMessage {
	type fieldValue struct {
		ID           string `json:"id"`
		Label        string `json:"label"`
		Type         string `json:"type,omitempty"`
		ResourceType string `json:"resource_type,omitempty"`
		Value        any    `json:"value"`
	}
	data, err := common.UIDataOf(card)
	if err != nil {
		return nil
	}
	input := struct {
		Title    string       `json:"title"`
		ActionID string       `json:"action_id"`
		Fields   []fieldValue `json:"fields"`
	}{Title: card.Title, ActionID: actionID, Fields: []fieldValue{}}
	for _, field := range data.Fields {
		if value, ok := values[field.ID]; ok {
			input.Fields = append(input.Fields, fieldValue{ID: field.ID, Label: field.Label, Type: field.Type, ResourceType: field.ResourceType, Value: value})
		}
	}
	for _, question := range data.Questions {
		if value, ok := values[question.ID]; ok {
			input.Fields = append(input.Fields, fieldValue{ID: question.ID, Label: question.Title, Type: question.Type, Value: value})
		}
	}
	return common.JSON(input)
}

// Raw submitted values belong in ai_ui_submission and the transient model input.
func uiSubmissionMessage(card models.UICard, actionID string) string {
	for _, action := range card.Actions {
		if action.ID == actionID {
			return card.Title + " · " + action.Label
		}
	}
	return card.Title
}

func (s *AgentService) RetryUISubmission(ctx context.Context, user, auth, runID, cardID, submissionID, requestID string, cfg *types.AIAssistantRuntime) (*models.UISubmissionResult, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -cfg.HistoryRetentionDays)
	card, err := s.cardForUser(ctx, user, auth, runID, cardID, cutoff)
	if err != nil {
		return nil, err
	}
	if card.Submission == nil || card.Submission.SubmissionID != submissionID {
		return nil, common.ErrRequestConflict
	}
	values, actionID, err := s.Store.UISubmissionValues(ctx, user, submissionID)
	if err != nil {
		return nil, err
	}
	return s.startUICardRun(ctx, user, auth, *card, models.UISubmissionRequest{RequestID: requestID, ActionID: actionID, Values: values}, submissionID, cfg, cutoff)
}

func (s *AgentService) UIResourceOptions(ctx context.Context, user, auth, runID, cardID, fieldID, query, cursor string, parents map[string]any, cutoff time.Time) ([]models.UIResourceOption, string, error) {
	card, err := s.cardForUser(ctx, user, auth, runID, cardID, cutoff)
	if err != nil {
		return nil, "", err
	}
	if (card.Status != "pending" && card.Status != "submitted") || card.Type != "resource_selector" {
		return nil, "", common.ErrRequestConflict
	}
	d, err := common.UIDataOf(*card)
	if err != nil {
		return nil, "", err
	}
	var field *models.UIField
	for i := range d.Fields {
		if d.Fields[i].ID == fieldID {
			field = &d.Fields[i]
		}
	}
	if field == nil {
		return nil, "", common.ErrInvalidRequest
	}
	if len(query) > 128 {
		return nil, "", common.ErrInvalidRequest
	}
	offset := 0
	if cursor != "" {
		offset, err = strconv.Atoi(cursor)
		if err != nil || offset < 0 || offset > 100000 {
			return nil, "", common.ErrInvalidCursor
		}
	}
	parent := models.UIResource{}
	if field.ParentFieldID != "" {
		raw, ok := parents[field.ParentFieldID]
		if !ok {
			return []models.UIResourceOption{}, "", nil
		}
		if json.Unmarshal(common.JSON(raw), &parent) != nil || parent.ProjectKey == "" {
			return nil, "", common.ErrInvalidRequest
		}
		if err := s.Platform.authorizeContext(ctx, auth, models.PageContext{ProjectKey: parent.ProjectKey, EnvName: parent.EnvName, Production: parent.Production}); err != nil {
			return nil, "", err
		}
	}
	var raw json.RawMessage
	switch field.ResourceType {
	case "project":
		raw, err = s.Platform.business(ctx, auth, http.MethodGet, "/api/v1/picket/projects", url.Values{"verbosity": {"detailed"}, "ungrouped": {"false"}}, nil)
	case "environment":
		raw, err = s.Platform.business(ctx, auth, http.MethodGet, "/api/environment/environments", url.Values{"projectName": {parent.ProjectKey}, "production": {strconv.FormatBool(field.Production)}}, nil)
	case "service":
		if parent.EnvName == "" || parent.Production != field.Production {
			return nil, "", common.ErrInvalidRequest
		}
		raw, err = s.Platform.business(ctx, auth, http.MethodGet, "/api/environment/environments/"+url.PathEscape(parent.EnvName), url.Values{"projectName": {parent.ProjectKey}, "production": {strconv.FormatBool(parent.Production)}}, nil)
	default:
		return nil, "", common.ErrInvalidRequest
	}
	if err != nil {
		return nil, "", err
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, "", common.ErrInvalidRequest
	}
	items := []models.UIResourceOption{}
	seen := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, v := range x {
				walk(v)
			}
		case map[string]any:
			key, label := "", ""
			keys := []string{"project_name", "product_name"}
			if field.ResourceType == "environment" {
				keys = []string{"env_name", "name"}
			}
			if field.ResourceType == "service" {
				keys = []string{"service_name"}
			}
			for _, k := range keys {
				if name, ok := x[k].(string); ok && name != "" {
					key = name
					break
				}
			}
			if key == "" && field.ResourceType == "project" && x["projects"] == nil && x["sub_projects"] == nil && x["children"] == nil {
				key, _ = x["name"].(string)
			}
			if key != "" && !seen[key] {
				label = key
				for _, k := range []string{"alias", "display_name"} {
					if v, ok := x[k].(string); ok && v != "" {
						label = v
						break
					}
				}
				resource := parent
				if field.ResourceType == "project" {
					resource = models.UIResource{ProjectKey: key}
				} else if field.ResourceType == "environment" {
					resource.EnvName = key
					resource.Production = field.Production
				} else {
					resource.ServiceName = key
				}
				if strings.Contains(strings.ToLower(label+" "+key), strings.ToLower(query)) {
					items = append(items, models.UIResourceOption{Label: label, Value: key, Resource: resource})
				}
				seen[key] = true
				return
			}
			for _, k := range []string{"projects", "sub_projects", "children", "result", "items", "data", "list", "groups", "services"} {
				if sub, ok := x[k]; ok {
					walk(sub)
				}
			}
		}
	}
	walk(value)
	sort.Slice(items, func(i, j int) bool { return items[i].Value < items[j].Value })
	authorized := items[:0]
	for _, item := range items {
		if s.Platform.authorizeContext(ctx, auth, models.PageContext{ProjectKey: item.Resource.ProjectKey, EnvName: item.Resource.EnvName, Production: item.Resource.Production}) == nil {
			authorized = append(authorized, item)
		}
	}
	items = authorized
	if offset >= len(items) {
		return []models.UIResourceOption{}, "", nil
	}
	end := offset + 50
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	} else {
		end = len(items)
	}
	return items[offset:end], next, nil
}
func (s *AgentService) validateUIResources(ctx context.Context, user, auth string, card models.UICard, values map[string]any, cutoff time.Time) ([]models.ResourceScope, error) {
	d, _ := common.UIDataOf(card)
	var scopes []models.ResourceScope
	for _, f := range d.Fields {
		if values[f.ID] == nil {
			continue
		}
		wanted := []any{values[f.ID]}
		if f.Multiple {
			wanted, _ = values[f.ID].([]any)
		}
		seen := map[models.UIResource]bool{}
		for _, v := range wanted {
			var r models.UIResource
			decoder := json.NewDecoder(strings.NewReader(string(common.JSON(v))))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&r) != nil || r.ProjectKey == "" || seen[r] {
				return nil, common.ErrInvalidRequest
			}
			seen[r] = true
			// Match against fresh, permission-checked options, including parent and production identity.
			needle := r.ProjectKey
			if f.ResourceType == "environment" {
				needle = r.EnvName
			}
			if f.ResourceType == "service" {
				needle = r.ServiceName
			}
			if err := matchUIResource(r, func(cursor string) ([]models.UIResourceOption, string, error) {
				return s.UIResourceOptions(ctx, user, auth, card.RunID, card.ID, f.ID, needle, cursor, values, cutoff)
			}); err != nil {
				return nil, err
			}
			scopes = append(scopes, models.ResourceScope{PageContext: models.PageContext{ProjectKey: r.ProjectKey, EnvName: r.EnvName, Production: r.Production}})
		}
	}
	return scopes, nil
}

func matchUIResource(wanted models.UIResource, load func(string) ([]models.UIResourceOption, string, error)) error {
	cursor := ""
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		if seen[cursor] {
			break
		}
		seen[cursor] = true
		items, next, err := load(cursor)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Resource == wanted {
				return nil
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}
	return fmt.Errorf("%w: resource selection changed or is not permitted", common.ErrInvalidRequest)
}
