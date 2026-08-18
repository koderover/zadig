package terminalaudit

import (
	"bufio"
	"context"
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

const (
	// writeQueueCapacity bounds the async write buffer so that terminal I/O is
	// never blocked by slow object-storage uploads. When the queue overflows we
	// degrade the recording rather than applying backpressure to the terminal.
	writeQueueCapacity = 8192
	// closeWriterTimeout bounds how long Close waits for the writer goroutine to
	// flush buffered events and close the upload pipe.
	closeWriterTimeout = 5 * time.Second
	// closePersistTimeout bounds how long Close waits for pending command
	// persistence goroutines.
	closePersistTimeout = 5 * time.Second
	// closeUploadTimeout bounds how long Close waits for the object-storage
	// upload to finish before abandoning it.
	closeUploadTimeout = 10 * time.Second
	// auditStorageLookupTimeout bounds the default storage lookup during audit initialization.
	auditStorageLookupTimeout = 5 * time.Second
)

type asciicastRecorder struct {
	mu          sync.Mutex
	errMu       sync.Mutex
	persistWG   sync.WaitGroup
	session     *models.TerminalSession
	startedAt   time.Time
	inputMask   *streamSanitizer
	outputMask  *streamSanitizer
	extractor   *CommandExtractor
	writer      *bufio.Writer
	pipeWriter  *io.PipeWriter
	writeCh     chan []byte
	writerDone  chan struct{}
	uploadDone  chan struct{}
	storageID   string
	bucket      string
	objectKey   string
	fileSize    atomic.Int64
	recordErr   error
	degraded    atomic.Bool
	closed      bool
	closeOnce   sync.Once
	closeErr    error
	sessionColl *commonrepo.TerminalSessionColl
	commandColl *commonrepo.TerminalCommandColl
	live        *livePublisher
}

type castHeader struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Env       map[string]string `json:"env,omitempty"`
	Title     string            `json:"title,omitempty"`
}

func newRecorder(meta *SessionMeta) (*asciicastRecorder, error) {
	if meta == nil {
		return nil, fmt.Errorf("terminal session meta is nil")
	}
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), auditStorageLookupTimeout)
	defer cancel()
	storage, err := s3service.FindDefaultS3WithContext(ctx)
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
		writeCh:     make(chan []byte, writeQueueCapacity),
		writerDone:  make(chan struct{}),
		uploadDone:  uploadDone,
		storageID:   storageID,
		bucket:      storage.Bucket,
		objectKey:   session.ObjectKey,
		sessionColl: sessionColl,
		commandColl: commonrepo.NewTerminalCommandColl(),
		live:        newLivePublisher(session.SessionID),
	}
	recorder.writer = bufio.NewWriter(&countingWriter{
		writer: pipeWriter,
		size:   &recorder.fileSize,
	})
	go func() {
		defer close(uploadDone)
		defer pipeReader.Close()
		if err := client.UploadReader(storage.Bucket, pipeReader, session.ObjectKey, "application/octet-stream"); err != nil {
			recorder.degrade(err)
		}
	}()
	// Write the header synchronously before the writer goroutine starts so that
	// there is only ever a single writer touching bufio.Writer.
	if err := recorder.writeHeader(normalizeDimension(meta.InitialCols, defaultCols), normalizeDimension(meta.InitialRows, defaultRows)); err != nil {
		recorder.live.close()
		_ = pipeWriter.CloseWithError(err)
		_ = sessionColl.CloseSession(&commonrepo.CloseSessionArgs{
			SessionID:       session.SessionID,
			Status:          models.TerminalSessionStatusFailed,
			EndedAt:         time.Now().Unix(),
			DurationSeconds: 0,
			StorageID:       storageID,
			Bucket:          storage.Bucket,
			ObjectKey:       session.ObjectKey,
			FileSize:        recorder.fileSize.Load(),
			ErrorMessage:    err.Error(),
		})
		return nil, err
	}
	go recorder.runWriter()
	log.Infof("create terminal audit recorder success, sessionID=%s storageID=%s bucket=%s objectKey=%s", session.SessionID, storageID, storage.Bucket, session.ObjectKey)
	return recorder, nil
}

// runWriter is the sole writer to bufio.Writer after startup. It drains the
// bounded queue into object storage and flushes/closes the upload pipe when the
// queue is closed by Close.
func (r *asciicastRecorder) runWriter() {
	defer close(r.writerDone)
	for line := range r.writeCh {
		if r.degraded.Load() {
			continue
		}
		if _, err := r.writer.Write(line); err != nil {
			r.degrade(err)
		}
	}
	if !r.degraded.Load() {
		if err := r.writer.Flush(); err != nil {
			r.degrade(err)
		}
	}
	if err := r.pipeWriter.Close(); err != nil {
		r.setRecordErr(err)
	}
}

func (r *asciicastRecorder) RecordInput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.degraded.Load() {
		return
	}
	r.recordInput(r.inputMask.Mask(data))
}

func (r *asciicastRecorder) RecordOutput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.degraded.Load() {
		return
	}
	r.recordOutput(r.outputMask.Mask(data))
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
	if r.closed || r.degraded.Load() {
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
			r.degrade(err)
			return
		}
		if err := r.sessionColl.UpdateActivity(r.session.SessionID, commandCount, activityAt); err != nil {
			r.degrade(err)
		}
	}(commandModels, int64(len(commands)), now)
}

func (r *asciicastRecorder) Close(status models.TerminalSessionStatus) error {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		r.closed = true
		if !r.degraded.Load() {
			r.recordInput(r.inputMask.Flush())
			r.recordOutput(r.outputMask.Flush())
			r.persistCommands(r.extractor.flush())
		}
		close(r.writeCh)
		r.mu.Unlock()

		// Bounded wait for the writer goroutine to flush buffered events and
		// close the upload pipe. Terminal shutdown must never block on storage.
		select {
		case <-r.writerDone:
		case <-time.After(closeWriterTimeout):
			r.degrade(fmt.Errorf("terminal audit writer flush timed out for session %s", r.session.SessionID))
			_ = r.pipeWriter.CloseWithError(fmt.Errorf("terminal audit writer flush deadline exceeded"))
		}

		r.live.close()

		persistDone := make(chan struct{})
		go func() {
			r.persistWG.Wait()
			close(persistDone)
		}()
		select {
		case <-persistDone:
		case <-time.After(closePersistTimeout):
			r.degrade(fmt.Errorf("terminal audit command persistence timed out for session %s", r.session.SessionID))
		}

		endedAt := time.Now().Unix()
		durationSeconds := int64(time.Since(r.startedAt).Seconds())
		if r.uploadDone != nil {
			select {
			case <-r.uploadDone:
			case <-time.After(closeUploadTimeout):
				r.degrade(fmt.Errorf("terminal audit upload timed out for session %s", r.session.SessionID))
				_ = r.pipeWriter.CloseWithError(fmt.Errorf("terminal audit upload deadline exceeded"))
			}
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
		r.degrade(err)
		return
	}
	select {
	case r.writeCh <- append(line, '\n'):
		if code == "o" {
			r.live.publish(string(line))
		}
	default:
		r.degrade(fmt.Errorf("terminal audit write buffer full for session %s, dropping recording", r.session.SessionID))
	}
}

func (r *asciicastRecorder) degrade(err error) {
	if err == nil {
		return
	}
	r.setRecordErr(err)
	r.degraded.Store(true)
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

// normalizeDimension applies the shared terminal-size defaulting rule.
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

// buildObjectKey defines the date-partitioned object-storage path for a cast file.
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
