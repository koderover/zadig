package terminalaudit

import (
	"errors"
	"fmt"
	"math"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
)

const (
	defaultTerminalAuditPageSize int64 = 20
	maxTerminalAuditPageSize     int64 = 100
)

func ListSessions(args *models.TerminalSessionListArgs) (*SessionListResponse, error) {
	if args == nil {
		args = &models.TerminalSessionListArgs{}
	}
	if err := normalizePagination(&args.PageNum, &args.PageSize); err != nil {
		return nil, err
	}
	sessions, total, err := commonrepo.NewTerminalSessionColl().List(args)
	if err != nil {
		return nil, err
	}
	return &SessionListResponse{Total: total, Sessions: sessions}, nil
}

func GetSession(sessionID string) (*models.TerminalSession, error) {
	session, err := commonrepo.NewTerminalSessionColl().FindBySessionID(sessionID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, e.NewWithDesc(e.ErrNotFound, "terminal session not found")
	}
	return session, err
}

func ListCommands(args *models.TerminalCommandListArgs) (*CommandListResponse, error) {
	if args == nil {
		args = &models.TerminalCommandListArgs{}
	}
	if err := normalizePagination(&args.PageNum, &args.PageSize); err != nil {
		return nil, err
	}
	commands, total, err := commonrepo.NewTerminalCommandColl().List(args)
	if err != nil {
		return nil, err
	}
	return &CommandListResponse{Total: total, Commands: commands}, nil
}

func GetCastStream(sessionID string) (*CastFileStream, error) {
	session, err := GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session.ObjectKey == "" {
		return nil, e.NewWithDesc(e.ErrNotFound, "cast file is not available")
	}

	store, err := getSessionStorage(session)
	if err != nil {
		return nil, err
	}
	client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Region, store.Insecure, store.Provider)
	if err != nil {
		return nil, err
	}
	bucket := session.Bucket
	if bucket == "" {
		bucket = store.Bucket
	}
	object, err := client.GetFile(bucket, session.ObjectKey, &s3tool.DownloadOption{IgnoreNotExistError: false, RetryNum: 2})
	if err != nil {
		return nil, err
	}
	return &CastFileStream{Body: object.Body, FileSize: session.FileSize}, nil
}

func TerminateSession(sessionID string) error {
	session, err := GetSession(sessionID)
	if err != nil {
		return err
	}
	if session.Status != models.TerminalSessionStatusRunning {
		return fmt.Errorf("terminal session %s is not running", sessionID)
	}
	subscribers, err := cache.NewRedisCache(config.RedisCommonCacheTokenDB()).PublishCount(liveTerminateChannel(sessionID), liveMessageTerminate)
	if err != nil {
		return err
	}
	if subscribers == 0 {
		return fmt.Errorf("terminal session %s is not active", sessionID)
	}
	return nil
}

// WatchSession subscribes to encoded asciicast frames for a running session.
func WatchSession(sessionID string) (<-chan string, func(), error) {
	session, err := GetSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	if session.Status != models.TerminalSessionStatusRunning {
		return nil, nil, e.NewWithDesc(e.ErrNotFound, "terminal session is not live")
	}
	return subscribeToLiveFrames(sessionID)
}

func normalizePagination(pageNum, pageSize *int64) error {
	if *pageNum <= 0 {
		*pageNum = 1
	}
	if *pageSize <= 0 {
		*pageSize = defaultTerminalAuditPageSize
	}
	if *pageSize > maxTerminalAuditPageSize {
		*pageSize = maxTerminalAuditPageSize
	}
	// Guard the skip calculation in repository List methods against int64 overflow.
	if *pageNum-1 > math.MaxInt64 / *pageSize {
		return e.NewWithDesc(e.ErrInvalidParam, "pageNum is too large")
	}
	return nil
}

func getSessionStorage(session *models.TerminalSession) (*s3service.S3, error) {
	if session.StorageID == internalStorageID {
		return s3service.FindInternalS3(), nil
	}
	if session.StorageID != "" {
		return s3service.FindS3ById(session.StorageID)
	}
	return s3service.FindDefaultS3()
}
