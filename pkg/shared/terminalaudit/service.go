package terminalaudit

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	s3tool "github.com/koderover/zadig/v2/pkg/tool/s3"
)

func ListSessions(args *models.TerminalSessionListArgs) (*SessionListResponse, error) {
	if args == nil {
		args = &models.TerminalSessionListArgs{}
	}
	normalizePagination(&args.PageNum, &args.PageSize)
	sessions, total, err := commonrepo.NewTerminalSessionColl().List(args)
	if err != nil {
		return nil, err
	}
	return &SessionListResponse{Total: total, Sessions: sessions}, nil
}

func GetSession(sessionID string) (*models.TerminalSession, error) {
	return commonrepo.NewTerminalSessionColl().FindBySessionID(sessionID)
}

func ListCommands(args *models.TerminalCommandListArgs) (*CommandListResponse, error) {
	if args == nil {
		args = &models.TerminalCommandListArgs{}
	}
	normalizePagination(&args.PageNum, &args.PageSize)
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
		return nil, e.ErrNotFound.AddDesc("cast file is not available")
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
	if object == nil {
		return nil, e.ErrNotFound.AddDesc("cast file not found")
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
	return TerminateActiveSession(sessionID)
}

func normalizePagination(pageNum, pageSize *int64) {
	if pageNum == nil || pageSize == nil {
		return
	}
	if *pageNum <= 0 {
		*pageNum = 1
	}
	if *pageSize <= 0 {
		*pageSize = 20
	}
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
