package terminalaudit

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
	"github.com/koderover/zadig/v2/pkg/util"
)

const internalStorageID = "__internal_default__"

type asciicastRecorder struct {
	mu            sync.Mutex
	errMu         sync.Mutex
	persistWG     sync.WaitGroup
	session       *models.TerminalSession
	startedAt     time.Time
	inputMask     *streamSanitizer
	outputMask    *streamSanitizer
	extractor     *CommandExtractor
	writer        *bufio.Writer
	pipeWriter    *io.PipeWriter
	uploadDone    chan struct{}
	storageID     string
	bucket        string
	objectKey     string
	fileSize      atomic.Int64
	recordErr     error
	closed        bool
	closeOnce     sync.Once
	closeErr      error
	terminate     func()
	terminateOnce sync.Once
	sessionColl   *commonrepo.TerminalSessionColl
	commandColl   *commonrepo.TerminalCommandColl
	live          *livePublisher
}

type castHeader struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Env       map[string]string `json:"env,omitempty"`
	Title     string            `json:"title,omitempty"`
}

func newRecorder(meta *SessionMeta, terminate func()) (*asciicastRecorder, error) {
	if meta == nil {
		return nil, fmt.Errorf("terminal session meta is nil")
	}
	startedAt := time.Now()
	storage, err := s3service.FindDefaultS3()
	if err != nil {
		return nil, err
	}
	sessionID := util.UUID()
	storageID := internalStorageID
	if !storage.ID.IsZero() {
		storageID = storage.ID.Hex()
	}
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
	uploadDone := make(chan struct{})

	recorder := &asciicastRecorder{
		session:     session,
		startedAt:   startedAt,
		inputMask:   newStreamSanitizer(meta.Secrets),
		outputMask:  newStreamSanitizer(meta.Secrets),
		extractor:   &CommandExtractor{},
		pipeWriter:  pipeWriter,
		uploadDone:  uploadDone,
		storageID:   storageID,
		bucket:      storage.Bucket,
		objectKey:   session.ObjectKey,
		sessionColl: sessionColl,
		commandColl: commonrepo.NewTerminalCommandColl(),
		live:        newLivePublisher(session.SessionID),
		terminate:   terminate,
	}
	recorder.writer = bufio.NewWriter(&countingWriter{
		writer: pipeWriter,
		size:   &recorder.fileSize,
	})
	go func() {
		defer close(uploadDone)
		defer pipeReader.Close()
		if err := client.UploadReader(storage.Bucket, pipeReader, session.ObjectKey, "application/octet-stream"); err != nil {
			recorder.fail(err)
		}
	}()
	if err := recorder.writeHeader(normalizeDimension(meta.InitialCols, defaultCols), normalizeDimension(meta.InitialRows, defaultRows)); err != nil {
		_ = recorder.Close(models.TerminalSessionStatusFailed)
		return nil, err
	}
	log.Infof("create terminal audit recorder success, sessionID=%s storageID=%s bucket=%s objectKey=%s", session.SessionID, storageID, storage.Bucket, session.ObjectKey)
	return recorder, nil
}

func (r *asciicastRecorder) RecordInput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.recordInput(r.inputMask.Write(data))
}

func (r *asciicastRecorder) RecordOutput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.recordOutput(r.outputMask.Write(data))
}

func (r *asciicastRecorder) recordInput(data string) {
	if data == "" {
		return
	}
	r.writeEvent("i", data)
	commands := r.extractor.Consume(data, time.Since(r.startedAt))
	r.persistCommands(commands)
}

func (r *asciicastRecorder) recordOutput(data string) {
	if data == "" {
		return
	}
	commands := r.extractor.ObserveOutput(data)
	r.writeEvent("o", data)
	r.persistCommands(commands)
}

func (r *asciicastRecorder) RecordResize(cols, rows uint16) {
	if cols == 0 || rows == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
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
			r.fail(err)
			return
		}
		if err := r.sessionColl.UpdateActivity(r.session.SessionID, commandCount, activityAt); err != nil {
			r.fail(err)
		}
	}(commandModels, int64(len(commands)), now)
}

func (r *asciicastRecorder) Close(status models.TerminalSessionStatus) error {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		r.recordInput(r.inputMask.Flush())
		r.recordOutput(r.outputMask.Flush())
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
		r.live.close()
		r.persistWG.Wait()

		endedAt := time.Now().Unix()
		durationSeconds := int64(time.Since(r.startedAt).Seconds())
		if r.uploadDone != nil {
			<-r.uploadDone
		}
		recordErr := r.getRecordErr()
		finalStatus := status
		if recordErr != nil && finalStatus == models.TerminalSessionStatusFinished {
			finalStatus = models.TerminalSessionStatusFailed
		}
		errorMessage := ""
		if recordErr != nil {
			errorMessage = recordErr.Error()
		}
		r.closeErr = errors.Join(recordErr, r.sessionColl.CloseSession(&commonrepo.CloseSessionArgs{
			SessionID:       r.session.SessionID,
			Status:          finalStatus,
			EndedAt:         endedAt,
			DurationSeconds: durationSeconds,
			StorageID:       r.storageID,
			Bucket:          r.bucket,
			ObjectKey:       r.objectKey,
			FileSize:        r.fileSize.Load(),
			ErrorMessage:    errorMessage,
		}))
		log.Infof("close terminal audit recorder, sessionID=%s status=%s fileSize=%d err=%v", r.session.SessionID, finalStatus, r.fileSize.Load(), r.closeErr)
	})
	return r.closeErr
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
	line, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := r.writer.Write(append(line, '\n')); err != nil {
		return err
	}
	if err := r.live.setHeader(string(line)); err != nil {
		return fmt.Errorf("save terminal live state: %w", err)
	}
	return nil
}

func (r *asciicastRecorder) writeEvent(code, data string) {
	offset := math.Round(time.Since(r.startedAt).Seconds()*1000) / 1000
	line, err := json.Marshal([]interface{}{offset, code, data})
	if err != nil {
		r.fail(err)
		return
	}
	if _, err := r.writer.Write(append(line, '\n')); err != nil {
		r.fail(err)
		return
	}
	r.live.publish(code, string(line))
}

func (r *asciicastRecorder) fail(err error) {
	if err == nil {
		return
	}
	r.setRecordErr(err)
	if r.terminate != nil {
		r.terminateOnce.Do(func() { go r.terminate() })
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
