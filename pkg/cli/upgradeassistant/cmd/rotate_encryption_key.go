package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/rotation"
	"github.com/koderover/zadig/v2/pkg/config"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	userconfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/microservice/user/core/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

func init() {
	cmd := &cobra.Command{
		Use: "rotate-encryption-key", Short: "Re-encrypt stored secrets during an offline maintenance window",
		Args: cobra.NoArgs, SilenceUsage: true,
		RunE: runKeyRotation,
	}
	cmd.Flags().String("old-key-file", "", "file containing the current AES key (16, 24 or 32 bytes; no newline)")
	cmd.Flags().String("new-key-file", "", "file containing the new AES key (16, 24 or 32 bytes; no newline)")
	cmd.Flags().Bool("dry-run", false, "validate records and remote hooks without any writes")
	cmd.Flags().Bool("allow-orphan-webhooks", false, "report hooks with no matching code host for manual rotation")
	_ = cmd.MarkFlagRequired("old-key-file")
	_ = cmd.MarkFlagRequired("new-key-file")
	rootCmd.AddCommand(cmd)
}

func runKeyRotation(cmd *cobra.Command, args []string) error {
	oldFile, _ := cmd.Flags().GetString("old-key-file")
	newFile, _ := cmd.Flags().GetString("new-key-file")
	dry, _ := cmd.Flags().GetBool("dry-run")
	allowOrphans, _ := cmd.Flags().GetBool("allow-orphan-webhooks")
	keys, err := rotation.ReadKeys(oldFile, newFile)
	if err != nil {
		return err
	}
	if config.MysqlUseDM() {
		return fmt.Errorf("key rotation currently supports MySQL only")
	}
	uri := viper.GetString(setting.ENVMongoDBConnectionString)
	database := viper.GetString(setting.ENVAslanDBName)
	if uri == "" || database == "" {
		return fmt.Errorf("MongoDB connection string and database are required")
	}
	ctx := cmd.Context()
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	mongotool.Init(connectCtx, uri)
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongotool.Close(closeCtx)
	}()
	if err := mongotool.Ping(connectCtx); err != nil {
		return fmt.Errorf("cannot connect to MongoDB")
	}

	initMysql()
	db, err := repository.DB.DB()
	if err != nil {
		return fmt.Errorf("cannot get MySQL user connection")
	}
	defer db.Close()
	cache := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(config.RedisHost(), strconv.Itoa(config.RedisPort())), Username: config.RedisUserName(), Password: config.RedisPassword(), DB: userconfig.RedisUserTokenDB()})
	defer cache.Close()
	if err := cache.Ping(connectCtx).Err(); err != nil {
		return fmt.Errorf("cannot connect to Redis user-token database")
	}
	proxies, err := commonrepo.NewProxyColl().List(&commonrepo.ProxyArgs{})
	if err != nil {
		return fmt.Errorf("cannot read code source proxy configuration")
	}
	proxy := ""
	if len(proxies) > 0 && proxies[0].EnableRepoProxy {
		proxy = proxies[0].GetProxyURL()
	}
	runner := &rotation.Runner{Mongo: mongotool.Database(database), SQL: db, Redis: cache, Keys: keys, Proxy: proxy, CallbackURL: gitservice.WebHookURL(), AllowOrphanWebhooks: allowOrphans}
	return runner.Run(ctx, dry, cmd.OutOrStdout())
}
