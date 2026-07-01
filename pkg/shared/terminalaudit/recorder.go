package terminalaudit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/shared/terminalio"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/util"
)

type TerminalRecorder interface {
	terminalio.Recorder
	SessionID() string
	Close(status models.TerminalSessionStatus) error
}

const internalStorageID = "__internal_default__"

type asciicastRecorder struct {
	mu          sync.Mutex
	errMu       sync.Mutex
	persistWG   sync.WaitGroup
	session     *models.TerminalSession
	startedAt   time.Time
	sanitizer   Sanitizer
	extractor   *CommandExtractor
	writer      *bufio.Writer
	encoder     *json.Encoder
	pipeWriter  *io.PipeWriter
	uploadDone  chan error
	storageID   string
	bucket      string
	objectKey   string
	fileSize    atomic.Int64
	recordErr   error
	closeOnce   sync.Once
	sessionColl *commonrepo.TerminalSessionColl
	commandColl *commonrepo.TerminalCommandColl
}

type castHeader struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Env       map[string]string `json:"env,omitempty"`
	Title     string            `json:"title,omitempty"`
}

func NewRecorder(meta *SessionMeta) (TerminalRecorder, error) {
	if meta == nil {
		return nil, fmt.Errorf("terminal session meta is nil")
	}
	startedAt := time.Now()
	storage, err := s3service.FindDefaultS3()
	if err != nil {
		return nil, err
	}
	sessionID := util.UUID()
	storageID := resolveStorageID(storage)
	objectKey := storage.GetObjectPath(buildObjectKey(meta.SessionType, startedAt, sessionID))
	session := &models.TerminalSession{
		SessionID:       sessionID,
		SessionType:     meta.SessionType,
		Status:          models.TerminalSessionStatusRunning,
		UserID:          meta.UserID,
		Username:        meta.Username,
		Account:         meta.Account,
		ProjectName:     meta.ProjectName,
		EnvName:         meta.EnvName,
		ServiceName:     meta.ServiceName,
		WorkflowName:    meta.WorkflowName,
		JobName:         meta.JobName,
		TaskID:          meta.TaskID,
		TargetName:      meta.TargetName,
		Protocol:        meta.Protocol,
		RemoteAddr:      meta.RemoteAddr,
		LoginAccount:    meta.LoginAccount,
		HostID:          meta.HostID,
		HostName:        meta.HostName,
		HostIP:          meta.HostIP,
		ClusterID:       meta.ClusterID,
		Namespace:       meta.Namespace,
		PodName:         meta.PodName,
		ContainerName:   meta.ContainerName,
		ClientIP:        meta.ClientIP,
		UserAgent:       meta.UserAgent,
		StartedAt:       startedAt.Unix(),
		LastActivityAt:  startedAt.Unix(),
		CreatedAt:       startedAt.Unix(),
		UpdatedAt:       startedAt.Unix(),
		CommandCount:    0,
		DurationSeconds: 0,
		StorageID:       storageID,
		Bucket:          storage.Bucket,
		ObjectKey:       objectKey,
	}
	sessionColl := commonrepo.NewTerminalSessionColl()
	if err := sessionColl.Create(session); err != nil {
		return nil, err
	}
	client, err := s3tool.NewClient(storage.Endpoint, storage.Ak, storage.Sk, storage.Region, storage.Insecure, storage.Provider)
	if err != nil {
		_ = sessionColl.CloseSession(&commonrepo.CloseSessionArgs{
			SessionID:       session.SessionID,
			Status:          models.TerminalSessionStatusFailed,
			EndedAt:         time.Now().Unix(),
			DurationSeconds: 0,
			StorageID:       storageID,
			Bucket:          storage.Bucket,
			ObjectKey:       session.ObjectKey,
			FileSize:        0,
			ErrorMessage:    err.Error(),
		})
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	uploadDone := make(chan error, 1)

	recorder := &asciicastRecorder{
		session:     session,
		startedAt:   startedAt,
		sanitizer:   NewSanitizer(meta.Secrets, meta.SecretEnvs),
		extractor:   NewCommandExtractor(),
		pipeWriter:  pipeWriter,
		uploadDone:  uploadDone,
		storageID:   storageID,
		bucket:      storage.Bucket,
		objectKey:   session.ObjectKey,
		sessionColl: sessionColl,
		commandColl: commonrepo.NewTerminalCommandColl(),
	}
	recorder.writer = bufio.NewWriter(&countingWriter{
		writer: pipeWriter,
		size:   &recorder.fileSize,
	})
	recorder.encoder = json.NewEncoder(recorder.writer)
	go func() {
		uploadDone <- client.UploadReader(storage.Bucket, pipeReader, session.ObjectKey, "application/octet-stream")
		close(uploadDone)
	}()
	if err := recorder.writeHeader(normalizeDimension(meta.InitialCols, defaultCols), normalizeDimension(meta.InitialRows, defaultRows)); err != nil {
		_ = recorder.Close(models.TerminalSessionStatusFailed)
		return nil, err
	}
	log.Infof("create terminal audit recorder success, sessionID=%s storageID=%s bucket=%s objectKey=%s", session.SessionID, storageID, storage.Bucket, session.ObjectKey)
	return recorder, nil
}

func (r *asciicastRecorder) SessionID() string {
	return r.session.SessionID
}

func (r *asciicastRecorder) RecordInput(data string) {
	sanitized := r.sanitizer.Mask(data)
	r.mu.Lock()
	if sanitized != "" {
		r.writeEvent("i", sanitized)
	}
	commands := r.extractor.Consume(sanitized, time.Since(r.startedAt))
	r.mu.Unlock()
	r.persistCommands(commands)
}

func (r *asciicastRecorder) RecordOutput(data string) {
	r.mu.Lock()
	if data == "" {
		r.mu.Unlock()
		return
	}
	commands := r.extractor.ObserveOutput(data)
	r.writeEvent("o", data)
	r.mu.Unlock()
	r.persistCommands(commands)
}

func (r *asciicastRecorder) RecordResize(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.writeEvent("r", fmt.Sprintf("%dx%d", cols, rows))
}

func (r *asciicastRecorder) persistCommands(commands []ExtractedCommand) {
	if len(commands) == 0 {
		return
	}
	now := time.Now().Unix()
	commandModels := make([]*models.TerminalCommand, 0, len(commands))
	for _, command := range commands {
		commandModels = append(commandModels, &models.TerminalCommand{
			SessionID:    r.session.SessionID,
			Seq:          command.Seq,
			Command:      command.Command,
			RiskLevel:    CommandRiskLevelAccepted,
			UserID:       r.session.UserID,
			Username:     r.session.Username,
			Account:      r.session.Account,
			ProjectName:  r.session.ProjectName,
			EnvName:      r.session.EnvName,
			TargetName:   r.session.TargetName,
			Protocol:     r.session.Protocol,
			RemoteAddr:   r.session.RemoteAddr,
			LoginAccount: r.session.LoginAccount,
			TimeOffsetMS: command.TimeOffsetMS,
			CreatedAt:    now,
		})
	}
	r.persistWG.Add(1)
	go func(commands []*models.TerminalCommand, commandCount int64, activityAt int64) {
		defer r.persistWG.Done()
		if err := r.commandColl.CreateMany(commands); err != nil {
			r.setRecordErr(err)
		}
		if err := r.sessionColl.UpdateActivity(r.session.SessionID, commandCount, activityAt); err != nil {
			r.setRecordErr(err)
		}
	}(commandModels, int64(len(commands)), now)
}

func (r *asciicastRecorder) Close(status models.TerminalSessionStatus) error {
	var closeErr error
	r.closeOnce.Do(func() {
		log.Infof("terminal audit recorder close start, sessionID=%s status=%s", r.session.SessionID, status)
		r.mu.Lock()
		if r.writer != nil {
			if err := r.writer.Flush(); err != nil {
				r.setRecordErr(err)
			}
		}
		if r.pipeWriter != nil {
			if err := r.pipeWriter.Close(); err != nil {
				r.setRecordErr(err)
			}
		}
		r.mu.Unlock()
		log.Infof("terminal audit recorder close flushed stream, sessionID=%s", r.session.SessionID)
		r.persistWG.Wait()
		log.Infof("terminal audit recorder close persist done, sessionID=%s", r.session.SessionID)

		endedAt := time.Now().Unix()
		durationSeconds := int64(time.Since(r.startedAt).Seconds())
		recordErr := r.getRecordErr()
		errorMessages := make([]string, 0)
		if recordErr != nil {
			errorMessages = append(errorMessages, recordErr.Error())
		}
		if r.uploadDone != nil {
			log.Infof("terminal audit recorder close wait upload, sessionID=%s", r.session.SessionID)
			if err := <-r.uploadDone; err != nil {
				errorMessages = append(errorMessages, err.Error())
			}
			log.Infof("terminal audit recorder close upload done, sessionID=%s fileSize=%d errors=%v", r.session.SessionID, r.fileSize.Load(), errorMessages)
		}

		finalStatus := status
		if len(errorMessages) > 0 && finalStatus == models.TerminalSessionStatusFinished {
			finalStatus = models.TerminalSessionStatusFailed
		}
		log.Infof("terminal audit recorder close update session, sessionID=%s finalStatus=%s endedAt=%d duration=%d fileSize=%d", r.session.SessionID, finalStatus, endedAt, durationSeconds, r.fileSize.Load())
		closeErr = r.sessionColl.CloseSession(&commonrepo.CloseSessionArgs{
			SessionID:       r.session.SessionID,
			Status:          finalStatus,
			EndedAt:         endedAt,
			DurationSeconds: durationSeconds,
			StorageID:       r.storageID,
			Bucket:          r.bucket,
			ObjectKey:       r.objectKey,
			FileSize:        r.fileSize.Load(),
			ErrorMessage:    strings.Join(errorMessages, "; "),
		})
		log.Infof("terminal audit recorder close finish, sessionID=%s err=%v", r.session.SessionID, closeErr)
	})
	return closeErr
}

func (r *asciicastRecorder) writeHeader(cols, rows int) error {
	header := castHeader{
		Version:   2,
		Width:     cols,
		Height:    rows,
		Timestamp: r.startedAt.Unix(),
		Env: map[string]string{
			"TERM": "xterm-256color",
		},
		Title: r.session.TargetName,
	}
	return r.encoder.Encode(header)
}

func (r *asciicastRecorder) writeEvent(code, data string) {
	offset := math.Round(time.Since(r.startedAt).Seconds()*1000) / 1000
	if err := r.encoder.Encode([]interface{}{offset, code, data}); err != nil {
		r.setRecordErr(err)
	}
}

func (r *asciicastRecorder) setRecordErr(err error) {
	if err == nil {
		return
	}
	r.errMu.Lock()
	defer r.errMu.Unlock()
	if r.recordErr == nil {
		r.recordErr = err
		return
	}
	r.recordErr = fmt.Errorf("%v; %w", r.recordErr, err)
}

func (r *asciicastRecorder) getRecordErr() error {
	r.errMu.Lock()
	defer r.errMu.Unlock()
	return r.recordErr
}

func normalizeDimension(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

type countingWriter struct {
	writer io.Writer
	size   *atomic.Int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if n > 0 {
		w.size.Add(int64(n))
	}
	return n, err
}

func buildObjectKey(sessionType models.TerminalSessionType, startedAt time.Time, sessionID string) string {
	return path.Join(
		"terminal-cast",
		string(sessionType),
		startedAt.Format("2006"),
		startedAt.Format("01"),
		startedAt.Format("02"),
		sessionID+".cast",
	)
}

func resolveStorageID(storage *s3service.S3) string {
	if storage == nil || storage.ID.IsZero() {
		return internalStorageID
	}
	return storage.ID.Hex()
}
