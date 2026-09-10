package job

import (
	"strings"
	"testing"
)

func TestBuildAIReviewScriptUsesMergedHead(t *testing.T) {
	script := buildAIReviewScript()

	if !strings.Contains(script, `TO_COMMIT="$(git rev-parse HEAD)"`) {
		t.Fatal("AI review script must capture the commit produced by the Git step")
	}
	if !strings.Contains(script, `--to "$TO_COMMIT"`) {
		t.Fatal("AI review script must review the merged commit")
	}
	if strings.Contains(script, "ZADIG_AI_REVIEW_PR") {
		t.Fatal("AI review script must not derive the review target from a PR environment variable")
	}
}
