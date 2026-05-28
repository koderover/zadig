package terminalaudit

import (
	"fmt"
	"sync"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

type activeSession struct {
	mu          sync.Mutex
	finalStatus models.TerminalSessionStatus
	terminate   func()
}

type activeSessionRegistry struct {
	sessions sync.Map
}

var registry = &activeSessionRegistry{}

func RegisterActiveSession(sessionID string, terminate func()) {
	registry.sessions.Store(sessionID, &activeSession{terminate: terminate})
}

func UnregisterActiveSession(sessionID string) {
	registry.sessions.Delete(sessionID)
}

func ResolveSessionStatus(sessionID string, defaultStatus models.TerminalSessionStatus) models.TerminalSessionStatus {
	session, ok := registry.load(sessionID)
	if !ok {
		return defaultStatus
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.finalStatus != "" {
		return session.finalStatus
	}
	return defaultStatus
}

func TerminateActiveSession(sessionID string) error {
	session, ok := registry.load(sessionID)
	if !ok {
		return fmt.Errorf("terminal session %s is not active", sessionID)
	}
	session.mu.Lock()
	session.finalStatus = models.TerminalSessionStatusAborted
	terminate := session.terminate
	session.mu.Unlock()
	if terminate != nil {
		terminate()
	}
	return nil
}

func (r *activeSessionRegistry) load(sessionID string) (*activeSession, bool) {
	value, ok := r.sessions.Load(sessionID)
	if !ok {
		return nil, false
	}
	session, ok := value.(*activeSession)
	return session, ok
}
