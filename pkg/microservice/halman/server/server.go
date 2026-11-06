package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	"github.com/go-resty/resty/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/koderover/zadig/v2/pkg/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	halmanconfig "github.com/koderover/zadig/v2/pkg/microservice/halman/config"
	ormrepo "github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/orm"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/service"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/server/rest"
	userclient "github.com/koderover/zadig/v2/pkg/shared/client/user"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

func Serve(ctx context.Context) error {
	log.Init(&log.Config{Level: config.LogLevel()})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cfg, err := halmanconfig.Load()
	if err != nil {
		return err
	}

	if config.MongoURI() == "" || config.MongoDatabase() == "" {
		return fmt.Errorf("缺少 MONGODB_CONNECTION_STRING 或 ASLAN_DB，不能读取 AI 配置")
	}

	initCtx, finishInit := context.WithTimeout(ctx, 30*time.Second)
	defer finishInit()
	mongotool.Init(initCtx, config.MongoURI())
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongotool.Close(closeCtx)
	}()
	if err := mongotool.Ping(initCtx); err != nil {
		return fmt.Errorf("连接 AI 配置 MongoDB 失败")
	}
	store, err := ormrepo.OpenStore(initCtx, halmanconfig.DatabaseDSN(), cfg.DBMaxConnections)
	if err != nil {
		return err
	}

	defer store.SQLDB.Close()
	err = store.Initialize(initCtx)
	finishInit()
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(cfg.UserAuthAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("用户鉴权服务配置无效")
	}

	defer conn.Close()
	users := userclient.New()
	users.SetRedirectPolicy(resty.NoRedirectPolicy())
	platform := &service.PlatformClient{
		Users:                  users,
		Auth:                   authv3.NewAuthorizationClient(conn),
		CheckEnterpriseLicense: commonutil.CheckZadigEnterpriseLicense,
		LoadRuntime:            commonrepo.NewSystemSettingColl().GetAIAssistantRuntime,
		HTTP: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	if err := store.InterruptRuns(ctx); err != nil {
		return fmt.Errorf("恢复 AI 运行状态失败")
	}

	agentService, err := service.NewAgentService(ctx, store, platform, cfg)
	if err != nil {
		return err
	}

	cleanupCtx, stopCleanup := context.WithCancel(ctx)
	agentService.StartMaintenance(cleanupCtx)
	defer func() { stopCleanup(); agentService.Close() }()
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           rest.NewEngine(agentService),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		WriteTimeout:      6 * time.Minute,
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	<-stopped
	return nil
}
