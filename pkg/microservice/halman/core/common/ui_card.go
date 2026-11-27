package common

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func UIDataOf(c models.UICard) (models.UIData, error) {
	var d models.UIData
	dec := json.NewDecoder(strings.NewReader(string(c.Data)))
	dec.DisallowUnknownFields()
	err := dec.Decode(&d)
	return d, err
}

var credentialField = regexp.MustCompile(`(?i)(password|passwd|secret|token|private.?key|api.?key|access.?key|credential|密码|令牌|密钥|凭据)`)

func ValidateUICard(c models.UICard) error {
	bad := func() error { return fmt.Errorf("invalid UI card") }
	if c.SchemaVersion != 1 || len(JSON(c)) > 128<<10 || len(c.Title) > 512 || strings.TrimSpace(c.Title) == "" {
		return bad()
	}
	d, err := UIDataOf(c)
	if err != nil {
		return bad()
	}
	switch c.Type {
	case "form", "resource_selector", "actions", "question":
	default:
		return bad()
	}
	if len(d.Fields) > 30 {
		return bad()
	}
	if (c.Type == "actions" && (len(d.Fields) > 0 || len(d.Questions) > 0)) || (c.Type != "question" && len(d.Questions) > 0) {
		return bad()
	}
	ids := map[string]models.UIField{}
	for _, f := range d.Fields {
		if f.ID == "" || len(f.ID) > 64 || ids[f.ID].ID != "" || len(f.Options) > 100 || len(f.Label) > 512 || strings.TrimSpace(f.Label) == "" || credentialField.MatchString(f.ID+" "+f.Label) {
			return bad()
		}
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			return bad()
		}
		if f.MinLength != nil && *f.MinLength < 0 || f.MaxLength != nil && *f.MaxLength < 0 {
			return bad()
		}
		if f.MinLength != nil && f.MaxLength != nil && *f.MinLength > *f.MaxLength {
			return bad()
		}
		if c.Type == "resource_selector" {
			switch f.ResourceType {
			case "project":
				if f.ParentFieldID != "" {
					return bad()
				}
			case "environment", "service":
				parent := ids[f.ParentFieldID]
				if parent.Multiple || (f.ResourceType == "environment" && parent.ResourceType != "project") || (f.ResourceType == "service" && (parent.ResourceType != "environment" || parent.Production != f.Production)) {
					return bad()
				}
			default:
				return bad()
			}
		} else {
			switch f.Type {
			case "text", "textarea", "number", "select", "radio", "checkbox", "switch":
			default:
				return bad()
			}
		}
		ids[f.ID] = f
		options := map[string]bool{}
		for _, o := range f.Options {
			if o.Value == "" || options[o.Value] {
				return bad()
			}
			options[o.Value] = true
		}
	}
	if (c.Type == "form" || c.Type == "resource_selector") && len(d.Fields) == 0 {
		return bad()
	}
	if c.Type == "question" {
		if len(d.Questions) < 1 || len(d.Questions) > 5 || len(d.Fields) != 0 {
			return bad()
		}
		questionIDs := map[string]bool{}
		for _, q := range d.Questions {
			if q.ID == "" || len(q.ID) > 64 || questionIDs[q.ID] || strings.TrimSpace(q.Title) == "" || len(q.Title) > 512 || len(q.Description) > 1024 || credentialField.MatchString(q.ID+" "+q.Title+" "+q.Description) {
				return bad()
			}
			questionIDs[q.ID] = true
			if q.Type == "text" {
				if len(q.Options) != 0 || q.AllowOther {
					return bad()
				}
			} else if q.Type == "single" || q.Type == "multiple" {
				if len(q.Options) < 2 || len(q.Options) > 8 {
					return bad()
				}
				optionIDs := map[string]bool{}
				for _, option := range q.Options {
					if option.Value == "" || option.Label == "" || optionIDs[option.Value] || len(option.Label) > 512 {
						return bad()
					}
					optionIDs[option.Value] = true
				}
			} else {
				return bad()
			}
		}
	}
	actions := map[string]bool{}
	for _, a := range c.Actions {
		if a.ID == "" || len(a.ID) > 64 || actions[a.ID] || strings.TrimSpace(a.Label) == "" || len(a.Label) > 512 {
			return bad()
		}
		actions[a.ID] = true
		switch a.Type {
		case "submit":
		case "prefill":
			if strings.TrimSpace(a.Prompt) == "" || len(a.Prompt) > 16000 {
				return bad()
			}
		case "navigate":
			if a.Target == nil || a.Target.ProjectKey == "" {
				return bad()
			}
		default:
			return bad()
		}
	}
	if c.Type == "actions" && len(c.Actions) == 0 {
		return bad()
	}
	if c.Type == "form" || c.Type == "resource_selector" {
		found := false
		for _, a := range c.Actions {
			found = found || a.Type == "submit"
		}
		if !found {
			return bad()
		}
	}
	if c.Type == "question" {
		if len(c.Actions) < 1 || len(c.Actions) > 2 || c.Actions[0].Type != "submit" || c.Actions[0].ID == "skip" {
			return bad()
		}
		if len(c.Actions) == 2 && (c.Actions[1].ID != "skip" || c.Actions[1].Type != "submit" || !models.AllQuestionsOptional(d.Questions)) {
			return bad()
		}
	}
	return nil
}

// An omitted, null or empty field has no value.
func UIValueEmpty(value any) bool {
	list, multiple := value.([]any)
	return value == nil || value == "" || multiple && len(list) == 0
}

func ValidateUIValues(c models.UICard, actionID string, values map[string]any) error {
	if err := ValidateUICard(c); err != nil {
		return err
	}
	allowed := false
	for _, a := range c.Actions {
		if a.ID == actionID && a.Type == "submit" {
			allowed = true
		}
	}
	if !allowed {
		return ErrInvalidRequest
	}
	d, _ := UIDataOf(c)
	if c.Type == "question" {
		if actionID == "skip" {
			if len(values) != 0 {
				return ErrInvalidRequest
			}
			return nil
		}
		return validateQuestionValues(d.Questions, values)
	}
	fields := map[string]bool{}
	for _, f := range d.Fields {
		fields[f.ID] = true
		v := values[f.ID]
		if UIValueEmpty(v) {
			if f.Required {
				return fmt.Errorf("%s is required", f.Label)
			}
			continue
		}
		if c.Type == "resource_selector" {
			if f.Multiple {
				a, ok := v.([]any)
				if !ok || len(a) > 100 {
					return ErrInvalidRequest
				}
				for _, item := range a {
					if _, ok := item.(map[string]any); !ok {
						return ErrInvalidRequest
					}
				}
			} else if _, ok := v.(map[string]any); !ok {
				return ErrInvalidRequest
			}
			continue
		}
		switch f.Type {
		case "number":
			n, ok := v.(float64)
			if !ok || math.IsNaN(n) || math.IsInf(n, 0) || (f.Min != nil && n < *f.Min) || (f.Max != nil && n > *f.Max) {
				return fmt.Errorf("%s is invalid", f.Label)
			}
		case "switch":
			if _, ok := v.(bool); !ok {
				return ErrInvalidRequest
			}
		case "text", "textarea":
			str, ok := v.(string)
			if !ok || len(str) > 16000 {
				return ErrInvalidRequest
			}
			n := utf8.RuneCountInString(str)
			if f.MinLength != nil && n < *f.MinLength || f.MaxLength != nil && n > *f.MaxLength {
				return ErrInvalidRequest
			}
		default:
			opts := map[string]bool{}
			for _, o := range f.Options {
				opts[o.Value] = true
			}
			if f.Type == "checkbox" {
				a, ok := v.([]any)
				if !ok {
					return ErrInvalidRequest
				}
				seen := map[string]bool{}
				for _, item := range a {
					str, ok := item.(string)
					if !ok || !opts[str] || seen[str] {
						return ErrInvalidRequest
					}
					seen[str] = true
				}
			} else {
				str, ok := v.(string)
				if !ok || !opts[str] {
					return ErrInvalidRequest
				}
			}
		}
	}
	for key := range values {
		if !fields[key] {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validateQuestionValues(questions []models.UIQuestion, values map[string]any) error {
	if len(values) > len(questions) {
		return ErrInvalidRequest
	}
	for _, q := range questions {
		value, present := values[q.ID]
		if !present || value == nil || value == "" {
			if q.Required {
				return fmt.Errorf("%s is required", q.Title)
			}
			continue
		}
		options := map[string]bool{}
		for _, option := range q.Options {
			options[option.Value] = true
		}
		validChoice := func(value any) (string, bool) {
			if selected, ok := value.(string); ok {
				return "option:" + selected, options[selected]
			}
			other, ok := value.(map[string]any)
			if !ok || !q.AllowOther || len(other) != 1 {
				return "", false
			}
			text, ok := other["other"].(string)
			return "other", ok && strings.TrimSpace(text) != "" && len(text) <= 2000
		}
		switch q.Type {
		case "text":
			text, ok := value.(string)
			if !ok || len(text) > 2000 || (q.Required && strings.TrimSpace(text) == "") {
				return ErrInvalidRequest
			}
		case "single":
			if _, ok := validChoice(value); !ok {
				return ErrInvalidRequest
			}
		case "multiple":
			selected, ok := value.([]any)
			if !ok || len(selected) > len(options)+1 || (q.Required && len(selected) == 0) {
				return ErrInvalidRequest
			}
			seen := map[string]bool{}
			for _, item := range selected {
				key, valid := validChoice(item)
				if !valid || seen[key] {
					return ErrInvalidRequest
				}
				seen[key] = true
			}
		}
	}
	for id := range values {
		found := false
		for _, q := range questions {
			found = found || q.ID == id
		}
		if !found {
			return ErrInvalidRequest
		}
	}
	return nil
}
