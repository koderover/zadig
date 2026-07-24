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
	recorder, err := newRecorder(meta, terminate)
	if err != nil {
		return nil, err
	}
	audit.Recorder = recorder
	audit.SessionID = recorder.SessionID()
	if err := registerActiveSession(audit.SessionID, terminate); err != nil {
		if closeErr := recorder.Close(models.TerminalSessionStatusFailed); closeErr != nil {
			log.Errorf("close terminal audit recorder after registration failure, sessionID=%s err=%v", audit.SessionID, closeErr)
		}
		return nil, err
	}
	log.Infof("register terminal audit session, sessionID=%s type=%s target=%s", audit.SessionID, meta.SessionType, meta.TargetName)
	return audit, nil
}

func (a *AuditSession) Close(finalStatus models.TerminalSessionStatus) error {
	if a == nil || a.Recorder == nil || a.SessionID == "" {
		return nil
	}
	resolvedStatus := resolveSessionStatus(a.SessionID, finalStatus)
	err := a.Recorder.Close(resolvedStatus)
	unregisterActiveSession(a.SessionID)
	return err
}
