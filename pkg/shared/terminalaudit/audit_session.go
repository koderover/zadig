package terminalaudit

import "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"

type AuditSession struct {
	Sanitizer Sanitizer
	Recorder  TerminalRecorder
	SessionID string
}

func NewAuditSession(meta *SessionMeta, terminate func()) (*AuditSession, error) {
	audit := &AuditSession{
		Sanitizer: NewSanitizer(meta.Secrets, meta.SecretEnvs),
	}
	recorder, err := NewRecorder(meta)
	if err != nil {
		return audit, err
	}
	audit.Recorder = recorder
	audit.SessionID = recorder.SessionID()
	RegisterActiveSession(audit.SessionID, terminate)
	return audit, nil
}

func (a *AuditSession) Close(finalStatus models.TerminalSessionStatus) error {
	if a == nil || a.Recorder == nil || a.SessionID == "" {
		return nil
	}
	resolvedStatus := ResolveSessionStatus(a.SessionID, finalStatus)
	err := a.Recorder.Close(resolvedStatus)
	UnregisterActiveSession(a.SessionID)
	return err
}
