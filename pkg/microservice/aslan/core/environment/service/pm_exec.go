package service

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
	toolssh "github.com/koderover/zadig/pkg/tool/ssh"
	"github.com/koderover/zadig/pkg/tool/wsconn"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func ConnectSshPmExec(c *gin.Context, username, envName, productName, ip string, log *zap.SugaredLogger) error {
	resp, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Address: ip,
	})
	if err != nil {
		log.Errorf("PrivateKey.Find ip %s error: %s", ip, err)
		return e.ErrGetPrivateKey
	}
	if resp.Status != setting.PMHostStatusNormal {
		return e.ErrLoginPm.AddDesc(fmt.Sprintf("host %s status %s,is not normal", ip, resp.Status))
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error(err)
		return e.ErrLoginPm.AddErr(err)
	}
	if resp.Port == 0 {
		resp.Port = setting.PMHostDefaultPort
	}
	sshCli, err := toolssh.NewSshCli([]byte(resp.PrivateKey), resp.UserName, resp.IP, resp.Port)
	if err != nil {
		conn.WriteMessage(1, []byte(err.Error()))
		conn.Close()
		return e.ErrLoginPm.AddErr(err)
	}
	sshClient := &wsconn.SshClient{
		// Username:   resp.UserName,
		// PrivateKey: resp.PrivateKey,
		// Ip:         resp.IP,
		// Port:       resp.Port,
		SshCli: sshCli,
	}

	if err := sshClient.GenerateWebTerminal(150, 40); err != nil {
		conn.WriteMessage(1, []byte(err.Error()))
		conn.Close()
	}
	sshClient.ConnectWs(conn)
	return nil
}
