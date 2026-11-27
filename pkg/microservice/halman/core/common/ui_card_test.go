package common

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestUICardRequiredQuestionsAndCredentialFields(t *testing.T) {
	card := models.UICard{
		SchemaVersion: 1, Type: "question", Title: "补充信息",
		Actions: []models.UIAction{{ID: "submit", Label: "提交", Type: "submit"}, {ID: "skip", Label: "跳过", Type: "submit"}},
	}
	question := models.UIQuestion{ID: "name", Title: "环境名称", Type: "text"}
	card.Data = JSON(models.UIData{Questions: []models.UIQuestion{question}})
	if err := ValidateUIValues(card, "skip", nil); err != nil {
		t.Fatal(err)
	}
	question.Required = true
	card.Data = JSON(models.UIData{Questions: []models.UIQuestion{question}})
	if err := ValidateUIValues(card, "skip", nil); err == nil {
		t.Fatal("required question accepted skip")
	}
	card.Actions = card.Actions[:1]
	if err := ValidateUIValues(card, "submit", map[string]any{"name": "test"}); err != nil {
		t.Fatal(err)
	}
	question.Title = "请填写 API Token"
	card.Data = JSON(models.UIData{Questions: []models.UIQuestion{question}})
	if err := ValidateUICard(card); err == nil {
		t.Fatal("credential collection was accepted")
	}
}

func TestUIRunFingerprintSeparatesRequestIdentity(t *testing.T) {
	base := models.RunRequest{RequestID: "same", Message: "提交"}
	text := RequestFingerprint("conversation", base)
	first := base
	first.UIContinuation = &models.UIContinuation{Card: models.UICard{ID: "first"}, Hash: "same-content"}
	second := first
	second.UIContinuation = &models.UIContinuation{Card: models.UICard{ID: "second"}, Hash: "same-content"}
	retry := first
	retry.UIContinuation = &models.UIContinuation{Card: models.UICard{ID: "first"}, SubmissionID: "submission", Hash: "same-content", Retry: true}
	a, b, c := RequestFingerprint("conversation", first), RequestFingerprint("conversation", second), RequestFingerprint("conversation", retry)
	if text == a || a == b || a == c || b == c {
		t.Fatal("different card operations shared the same request fingerprint")
	}
}
