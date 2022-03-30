package wsconn

import (
	"bufio"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/koderover/zadig/pkg/tool/log"
)

type ptyReqParam struct {
	Term     string
	Cols     uint32
	Rows     uint32
	Width    uint32
	Height   uint32
	Modelist string
}

type SshClient struct {
	SshSession *ssh.Session
	SshCli     *ssh.Client
	sshChan    ssh.Channel
}

func (sshCli *SshClient) GenerateWebTerminal(cols, rows uint32) error {
	session, err := sshCli.SshCli.NewSession()
	if err != nil {
		return err
	}
	sshCli.SshSession = session
	channel, inRequests, err := sshCli.SshCli.OpenChannel("session", nil)
	if err != nil {
		return err
	}
	sshCli.sshChan = channel
	go func() {
		for req := range inRequests {
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}()
	termModes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	var termModesList []byte
	for k, v := range termModes {
		kv := struct {
			Key byte
			Val uint32
		}{k, v}
		termModesList = append(termModesList, ssh.Marshal(&kv)...)
	}
	termModesList = append(termModesList, 0)
	reqParam := ptyReqParam{
		Term:     "xterm",
		Cols:     cols,
		Rows:     rows,
		Width:    uint32(cols * 8),
		Height:   uint32(cols * 8),
		Modelist: string(termModesList),
	}
	ok, err := channel.SendRequest("pty-req", true, ssh.Marshal(&reqParam))
	if !ok || err != nil {
		return err
	}
	ok, err = channel.SendRequest("shell", true, nil)
	if !ok || err != nil {
		return err
	}
	return nil
}

func (sshCli *SshClient) ConnectWs(ws *websocket.Conn) {
	go func() {
		for {
			_, p, err := ws.ReadMessage()
			if err != nil {
				return
			}
			_, err = sshCli.sshChan.Write(p)
			if err != nil {
				return
			}
		}
	}()

	go func() {
		bufferReader := bufio.NewReader(sshCli.sshChan)
		buffer := []byte{}
		ticker := time.NewTimer(time.Microsecond * 100)
		defer ticker.Stop()

		rChan := make(chan rune)
		go func() {
			defer sshCli.SshCli.Close()
			defer sshCli.sshChan.Close()

			for {
				r, size, err := bufferReader.ReadRune()
				if err != nil {
					log.Error(err)
					ws.WriteMessage(1, []byte("\033[31m websocket is closed\033[0m"))
					ws.Close()
					return
				}
				if size > 0 {
					rChan <- r
				}
			}
		}()

		for {
			select {
			case <-ticker.C:
				if len(buffer) != 0 {
					err := ws.WriteMessage(websocket.TextMessage, buffer)
					buffer = []byte{}
					if err != nil {
						log.Error(err)
						return
					}
				}
				ticker.Reset(time.Microsecond * 100)
			case dRune := <-rChan:
				if dRune != utf8.RuneError {
					byt := make([]byte, utf8.RuneLen(dRune))
					utf8.EncodeRune(byt, dRune)
					buffer = append(buffer, byt...)
				} else {
					buffer = append(buffer, []byte("@")...)
				}
			}
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			log.Error(err)
		}
	}()
}
