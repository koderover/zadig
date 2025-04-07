/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package remotedialer

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/buraksezer/consistent"
	"github.com/gorilla/websocket"
	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/pkg/errors"
)

var (
	errFailedAuth       = errors.New("failed authentication")
	errWrongMessageType = errors.New("wrong websocket message type")
)

type Authorizer func(req *http.Request) (clientKey string, authed bool, err error)
type ErrorWriter func(rw http.ResponseWriter, req *http.Request, code int, err error)

func DefaultErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	rw.WriteHeader(code)
	_, errWrite := rw.Write([]byte(err.Error()))
	if errWrite != nil {
		log.Warnf("Failed to write error: %s", errWrite)
	}
}

type ConnectRequestInfo struct {
	Method      string              `json:"method"`
	URL         string              `json:"url"`
	RemoteAddr  string              `json:"remoteAddr"`
	ClientKey   string              `json:"clientKey"`
	Timestamp   time.Time           `json:"timestamp"`
	Host        string              `json:"host"`
	UserAgent   string              `json:"userAgent"`
	Referer     string              `json:"referer"`
	Protocol    string              `json:"protocol"`
	Peer        bool                `json:"peer"`
	Header      map[string][]string `json:"headerKeys"`
	ContentType string              `json:"contentType"`
	ContentLen  int64               `json:"contentLen"`
}

type Server struct {
	PeerID                  string
	PeerToken               string
	ClientConnectAuthorizer ConnectAuthorizer
	authorizer              Authorizer
	errorWriter             ErrorWriter
	sessions                *sessionManager
	peers                   map[string]peer
	peerLock                sync.Mutex
	redisCache              *cache.RedisCache

	caCert        string
	httpTransport *http.Transport

	sync.Mutex
}

func New(auth Authorizer, errorWriter ErrorWriter) *Server {
	return &Server{
		peers:       map[string]peer{},
		authorizer:  auth,
		errorWriter: errorWriter,
		sessions:    newSessionManager(),
		redisCache:  cache.NewRedisCache(config.RedisCommonCacheTokenDB()),
	}
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	clientKey, authed, peer, err := s.auth(req)
	if err != nil {
		s.errorWriter(rw, req, 400, err)
		return
	}
	if !authed {
		s.errorWriter(rw, req, 401, errFailedAuth)
		return
	}

	// 保存请求信息到 Redis
	err = s.saveConnectRequest(clientKey, req)
	if err != nil {
		s.errorWriter(rw, req, 400, err)
		return
	}

	log.Infof("Handling backend connection request [%s]", clientKey)

	upgrader := websocket.Upgrader{
		HandshakeTimeout: 5 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
		Error:            s.errorWriter,
	}

	wsConn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		s.errorWriter(rw, req, 400, errors.Wrapf(err, "Error during upgrade for host [%v]", clientKey))
		return
	}

	session := s.sessions.add(clientKey, wsConn, peer)
	session.auth = s.ClientConnectAuthorizer
	defer s.sessions.remove(session)

	code, err := session.Serve(req.Context())
	if err != nil {
		// Hijacked so we can't write to the client
		log.Infof("error in remotedialer server [%d]: %v", code, err)
	}
}

func (s *Server) saveConnectRequest(clientKey string, req *http.Request) error {
	key := fmt.Sprintf("remotedialer:req:%s:%s", clientKey, time.Now().Format("20060102150405"))

	// 获取所有 header keys
	headerKeys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		headerKeys = append(headerKeys, k)
	}

	// 创建请求信息结构体
	reqInfo := ConnectRequestInfo{
		Method:      req.Method,
		URL:         req.URL.String(),
		RemoteAddr:  req.RemoteAddr,
		ClientKey:   clientKey,
		Timestamp:   time.Now(),
		Host:        req.Host,
		UserAgent:   req.UserAgent(),
		Referer:     req.Referer(),
		Protocol:    req.Proto,
		Header:      req.Header,
		ContentType: req.Header.Get("Content-Type"),
		ContentLen:  req.ContentLength,
	}

	// 将结构体序列化为 JSON
	jsonData, err := json.Marshal(reqInfo)
	if err != nil {
		return fmt.Errorf("Failed to marshal request info to JSON: %v", err)
	}

	// 保存到 Redis
	err = s.redisCache.Write(key, string(jsonData), 0)
	if err != nil {
		return fmt.Errorf("Failed to save request info to Redis: %v", err)
	}
	return nil
}

func (s *Server) auth(req *http.Request) (clientKey string, authed, peer bool, err error) {
	id := req.Header.Get(ID)
	token := req.Header.Get(Token)
	if id != "" && token != "" {
		// peer authentication
		s.peerLock.Lock()
		p, ok := s.peers[id]
		s.peerLock.Unlock()

		if ok && p.token == token {
			return id, true, true, nil
		}
	}

	id, authed, err = s.authorizer(req)
	return id, authed, false, err
}

func (r *Server) GetTransport(clusterCaCert string, clientKey string) (http.RoundTripper, error) {

	r.Lock()
	defer r.Unlock()

	transport := &http.Transport{}
	if clusterCaCert != "" {
		certBytes, err := base64.StdEncoding.DecodeString(clusterCaCert)
		if err != nil {
			return nil, err
		}
		certs := x509.NewCertPool()
		certs.AppendCertsFromPEM(certBytes)
		transport.TLSClientConfig = &tls.Config{
			RootCAs: certs,
		}
	}

	d := r.Dialer(clientKey)
	transport.DialContext = d
	transport.Proxy = http.ProxyFromEnvironment
	transport.IdleConnTimeout = 30 * time.Second

	r.caCert = clusterCaCert
	if r.httpTransport != nil {
		r.httpTransport.CloseIdleConnections()
	}
	r.httpTransport = transport

	return transport, nil
}

func (s *Server) Disconnect(clientKey string) {
	for _, session := range s.sessions.clients[clientKey] {
		session.CloseImmediately()
	}
}

func (s *Server) CleanSessions(old, new *consistent.Consistent) {
	if old == nil {
		return
	}

	s.sessions.Lock()
	defer s.sessions.Unlock()

	for clientID := range s.sessions.peers {
		oldMember := old.LocateKey([]byte(clientID))
		newMember := new.LocateKey([]byte(clientID))
		if oldMember.String() != newMember.String() {
			log.Debugf("Disconnect peer %s due to consistent hash change", clientID)
			s.Disconnect(clientID)
		}
	}
	for clientID := range s.sessions.clients {
		oldMember := old.LocateKey([]byte(clientID))
		newMember := new.LocateKey([]byte(clientID))
		if oldMember.String() != newMember.String() {
			log.Infof("Disconnect client %s due to consistent hash change", clientID)
			s.Disconnect(clientID)
		}
	}
}
