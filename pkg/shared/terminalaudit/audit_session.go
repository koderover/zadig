package terminalaudit

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type AuditSession struct {
	Recorder  TerminalRecorder
	SessionID string
}

func NewAuditSession(meta *SessionMeta, terminate func()) (*AuditSession, error) {
	audit := &AuditSession{}
	recorder, err := NewRecorder(meta)
	if err != nil {
		return nil, err
	}
	audit.Recorder = recorder
	audit.SessionID = recorder.SessionID()
	if err := RegisterActiveSession(audit.SessionID, terminate); err != nil {
		if closeErr := recorder.Close(models.TerminalSessionStatusFailed); closeErr != nil {
			log.Errorf("close terminal audit recorder after registration failure, sessionID=%s err=%v", audit.SessionID, closeErr)
		}
		return audit, err
	}
	log.Infof("register terminal audit session, sessionID=%s type=%s target=%s", audit.SessionID, meta.SessionType, meta.TargetName)
	return audit, nil
}

func (a *AuditSession) Close(finalStatus models.TerminalSessionStatus) error {
	if a == nil || a.Recorder == nil || a.SessionID == "" {
		return nil
	}
	resolvedStatus := ResolveSessionStatus(a.SessionID, finalStatus)
	log.Infof("close terminal audit session start, sessionID=%s finalStatus=%s resolvedStatus=%s", a.SessionID, finalStatus, resolvedStatus)
	err := a.Recorder.Close(resolvedStatus)
	UnregisterActiveSession(a.SessionID)
	log.Infof("close terminal audit session finish, sessionID=%s err=%v", a.SessionID, err)
	return err
}
