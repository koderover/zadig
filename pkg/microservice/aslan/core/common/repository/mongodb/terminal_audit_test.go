package mongodb

import (
	"testing"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestTerminalSessionCreateRejectsNilSession(t *testing.T) {
	if err := (&TerminalSessionColl{}).Create(nil); err == nil {
		t.Fatal("expected nil session error")
	}
}

func TestTerminalSessionCloseRejectsNilArgs(t *testing.T) {
	if err := (&TerminalSessionColl{}).CloseSession(nil); err == nil {
		t.Fatal("expected nil close arguments error")
	}
}

func TestTerminalCommandCreateManyRejectsNilCommand(t *testing.T) {
	commands := []*models.TerminalCommand{nil}
	if err := (&TerminalCommandColl{}).CreateMany(commands); err == nil {
		t.Fatal("expected nil command error")
	}
}
