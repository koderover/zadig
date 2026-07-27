package terminalaudit

import (
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

type AuditSession struct {
	Recorder  *asciicastRecorder
	SessionID string
}

func NewAuditSession(meta *SessionMeta, terminate func()) (*AuditSession, error) {
	recorder, err := newRecorder(meta)
	if err != nil {
		return nil, err
	}
	audit := &AuditSession{Recorder: recorder, SessionID: recorder.session.SessionID}
	if err := registerActiveSession(audit.SessionID, terminate); err != nil {
		// Live-watch/remote-terminate registration is best-effort. If it fails we
		// keep recording; only this session's live spectating is unavailable.
		log.Warnf("register terminal live session failed, recording continues, sessionID=%s err=%v", audit.SessionID, err)
		return audit, nil
	}
	log.Infof("register terminal audit session, sessionID=%s type=%s target=%s", audit.SessionID, meta.SessionType, meta.TargetName)
	return audit, nil
}

func (a *AuditSession) Close(finalStatus models.TerminalSessionStatus) error {
	if session, ok := registry.load(a.SessionID); ok {
		finalStatus = session.closeWithStatus(finalStatus)
		unregisterActiveSession(a.SessionID)
	}
	return a.Recorder.Close(finalStatus)
}
