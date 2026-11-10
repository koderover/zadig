package permission

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
)

func TestAIManagementPermissions(t *testing.T) {
	for _, verb := range []string{"", VerbGetAIManagement, VerbEditAIManagement} {
		actions := generateDefaultSystemActions()
		modifySystemAction(actions, verb)
		raw, err := json.Marshal(actions)
		if err != nil {
			t.Fatal(err)
		}
		var sdk userclient.SystemActions
		if err := json.Unmarshal(raw, &sdk); err != nil {
			t.Fatal(err)
		}
		if sdk.AIManagement == nil || sdk.AIManagement.View != (verb == VerbGetAIManagement) || sdk.AIManagement.Edit != (verb == VerbEditAIManagement) {
			t.Fatalf("unexpected SDK permissions for %q: %s", verb, raw)
		}
	}
	if slices.Contains(globalReadOnlySystemAction, VerbGetAIManagement) || slices.Contains(globalReadOnlySystemAction, VerbEditAIManagement) {
		t.Fatal("global read-only must not grant AI settings permissions")
	}
	if systemResourceActionAliasMap["AIManagement"] != "AI 能力" || !slices.Contains(systemResourceSequence, "AIManagement") {
		t.Fatal("AI permissions must appear in system resource definitions")
	}
	for _, path := range []string{"../../init/action_initialization.sql", "../../init/dm_action_initialization.sql"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{VerbGetAIManagement, VerbEditAIManagement, "AIManagement"} {
			if !strings.Contains(string(raw), value) {
				t.Fatalf("missing %s in %s", value, path)
			}
		}
	}
}
