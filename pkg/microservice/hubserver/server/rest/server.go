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

package rest

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	h "github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/handler"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/hubserver/core/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/remotedialer"
)

var Ready = false

type engine struct {
	*mux.Router
}

func NewEngine(handler *remotedialer.Server) *engine {
	s := &engine{}
	s.Router = mux.NewRouter()
	s.Router.UseEncodedPath()

	s.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			vars := mux.Vars(r)
			clientKey := vars["id"]
			if clientKey == "" {
				token := r.Header.Get(setting.Token)
				clusterID, err := crypto.AesDecrypt(token)
				if err != nil {
					log.Errorf("token is illegal %s: %v", token, err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				clientKey = clusterID
			}

			if _, err := mongodb.NewK8sClusterColl().Get(clientKey); err != nil {
				log.Errorf("unknown cluster, cluster id:%s, err:%v", clientKey, err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if clientKey != "" {
				// 使用一致性哈希选择目标 pod
				targetIP := service.GetNodeByKey(clientKey)
				if targetIP != "" {
					// 获取当前 pod 的 IP
					currentIP := os.Getenv("POD_IP")
					if currentIP == "" {
						log.Errorf("Failed to get pod IP from POD_IP env")
						http.Error(w, "Internal Server Error", http.StatusInternalServerError)
						return
					}

					// 检查目标 IP 是否与当前服务 IP 相同
					if targetIP == currentIP {
						next.ServeHTTP(w, r)
						return
					}

					log.Debugf("url: %+v, clientKey: %+v", r.URL.String(), clientKey)
					log.Debugf("targetIP: %s, currentIP: %s, forwarding to %s", targetIP, currentIP, targetIP)

					// 使用switch语句处理不同类型的请求，更加清晰
					switch {
					case isWebSocketRequest(r):
						// 处理WebSocket请求
						log.Infof("WebSocket request detected, handling proxy for WebSocket")
						handleWebSocketProxy(w, r, targetIP)

					case isSSERequest(r):
						// 处理Server-Sent Events请求
						log.Infof("SSE request detected, handling proxy for SSE")
						handleSSEProxy(w, r, targetIP)

					case shouldHandleAsStream(r):
						// 处理其他流式请求
						log.Infof("Generic streaming request detected, handling as stream")
						handleStreamingProxy(w, r, targetIP)

					default:
						// 处理普通HTTP请求
						log.Infof("handleRegularHTTPProxy")
						handleRegularHTTPProxy(w, r, targetIP)
					}
					return
				}
			} else {
				log.Errorf("clientKey is empty")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	s.injectRouters(handler)

	return s
}

func (s *engine) injectRouters(handler *remotedialer.Server) {
	r := s.Router

	r.Handle("/connect", handler)

	r.HandleFunc("/disconnect/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.Disconnect(handler, rw, req)
	})

	r.HandleFunc("/restore/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.Restore(rw, req)
	})

	r.HandleFunc("/hasSession/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.HasSession(handler, rw, req)
	})

	r.HandleFunc("/kube/{id}{path:.*}", func(rw http.ResponseWriter, req *http.Request) {
		h.Forward(handler, rw, req)
	})

	r.HandleFunc("/health", func(rw http.ResponseWriter, req *http.Request) {
		if Ready {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	s.Router = r
}

// isWebSocketRequest 检查请求是否为WebSocket请求
func isWebSocketRequest(r *http.Request) bool {
	// 检查常见的 WebSocket 握手头
	connectionHeader := r.Header.Get("Connection")
	upgradeHeader := r.Header.Get("Upgrade")

	return strings.ToLower(connectionHeader) == "upgrade" &&
		strings.ToLower(upgradeHeader) == "websocket"
}

// isSSERequest 检查请求是否为Server-Sent Events请求
func isSSERequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/event-stream")
}

// shouldHandleAsStream 检查请求是否应该作为流处理
func shouldHandleAsStream(r *http.Request) bool {
	// 检查请求头中是否有流式传输的标识
	if r.Header.Get("X-Stream") == "true" {
		return true
	}

	// 检查路径是否包含流式API的模式
	if strings.Contains(r.URL.Path, "/stream/") ||
		strings.Contains(r.URL.Path, "/logs/") ||
		strings.Contains(r.URL.Path, "/events/") {
		return true
	}

	// 检查Content-Type是否为流媒体内容
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "video/") ||
		strings.Contains(contentType, "audio/") {
		return true
	}

	return false
}

// handleWebSocketProxy 处理WebSocket连接的代理转发
func handleWebSocketProxy(w http.ResponseWriter, r *http.Request, targetIP string) {
	log := log.SugaredLogger()

	// 升级客户端连接到 WebSocket
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有源
		},
	}

	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("Failed to upgrade client connection: %v", err)
		http.Error(w, "Failed to upgrade to WebSocket protocol", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// 目标 WebSocket URL
	targetURL := url.URL{
		Scheme:   "ws",
		Host:     targetIP + ":26000",
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// 创建到目标服务器的 WebSocket 连接
	// 不使用原始请求的WebSocket特定头，而是让websocket包自己生成新的头
	targetHeaders := make(http.Header)

	// 只复制非WebSocket特定的头
	skipHeaders := []string{"Connection", "Upgrade", "Sec-WebSocket-Key",
		"Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Protocol"}

	for k, vs := range r.Header {
		if !slices.Contains(skipHeaders, k) {
			for _, v := range vs {
				targetHeaders.Add(k, v)
			}
		}
	}

	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 30 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	}

	targetConn, resp, err := dialer.Dial(targetURL.String(), targetHeaders)
	if err != nil {
		log.Errorf("Failed to connect to backend WebSocket: %v", err)
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Target response: %d %s %s", resp.StatusCode, resp.Status, string(body))

			// 通过WebSocket协议发送错误信息给客户端
			clientConn.WriteMessage(websocket.TextMessage,
				[]byte(fmt.Sprintf("Backend connection error: %v", err)))
			clientConn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Backend connection failed"))
		}
		return
	}
	defer targetConn.Close()

	errChan := make(chan error, 2)

	// 从客户端到目标
	go func() {
		for {
			messageType, msg, err := clientConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}

			err = targetConn.WriteMessage(messageType, msg)
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	// 从目标到客户端
	go func() {
		for {
			messageType, msg, err := targetConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}

			err = clientConn.WriteMessage(messageType, msg)
			if err != nil {
				errChan <- err
				return
			}
		}
	}()

	// 等待任一方向出错
	err = <-errChan
	if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		log.Warnf("WebSocket error: %v", err)
	}
}

// handleSSEProxy 处理SSE连接的代理转发
func handleSSEProxy(w http.ResponseWriter, r *http.Request, targetIP string) {
	log := log.SugaredLogger()

	// 创建转发请求
	proxyURL := &url.URL{
		Scheme:   "http",
		Host:     targetIP + ":26000",
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// 创建代理请求
	proxyReq, err := http.NewRequest(r.Method, proxyURL.String(), r.Body)
	if err != nil {
		log.Errorf("Failed to create proxy request for SSE: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 复制原始请求的 header
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 配置特别优化的HTTP客户端用于SSE
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 60 * time.Second, // 更长的KeepAlive时间
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 0, // 禁用响应头超时
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // 无超时
	}

	// 发送代理请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Errorf("Failed to forward SSE request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 设置SSE特定的头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 复制其他响应头
	for key, values := range resp.Header {
		if key != "Content-Type" && key != "Cache-Control" && key != "Connection" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	w.WriteHeader(resp.StatusCode)

	// 使用缓冲区读取和写入SSE数据
	buffer := make([]byte, 4096)
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Errorf("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := w.Write(buffer[:n])
			if writeErr != nil {
				log.Errorf("Error writing SSE data: %v", writeErr)
				return
			}
			flusher.Flush() // 确保数据立即发送到客户端
		}

		if err != nil {
			if err != io.EOF {
				log.Warnf("Error reading SSE data: %v", err)
			}
			break
		}
	}
}

// handleStreamingProxy 处理通用流式传输的代理转发
func handleStreamingProxy(w http.ResponseWriter, r *http.Request, targetIP string) {
	log := log.SugaredLogger()

	// 创建转发请求
	proxyURL := &url.URL{
		Scheme:   "http",
		Host:     targetIP + ":26000",
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// 创建代理请求
	proxyReq, err := http.NewRequest(r.Method, proxyURL.String(), r.Body)
	if err != nil {
		log.Errorf("Failed to create proxy request for streaming: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 复制原始请求的 header
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 配置针对流式传输优化的HTTP客户端
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 0, // 禁用响应头超时
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   0, // 无超时
	}

	// 发送代理请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Errorf("Failed to forward streaming request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 确保保持长连接和chunked编码
	w.Header().Set("Connection", "keep-alive")

	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 使用flusher来确保数据立即发送
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Errorf("Streaming not supported")
		return
	}

	// 使用较大的缓冲区提高性能
	buf := make([]byte, 8192)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				log.Errorf("Error writing streaming data: %v", writeErr)
				return
			}
			flusher.Flush() // 强制将数据推送到客户端
		}

		if err != nil {
			if err != io.EOF {
				log.Warnf("Error reading streaming data: %v", err)
			}
			break
		}
	}
}

// isStreamResponse 检查响应是否为流式响应
func isStreamResponse(resp *http.Response) bool {
	// 检查Transfer-Encoding是否为chunked
	if resp.TransferEncoding != nil && len(resp.TransferEncoding) > 0 {
		for _, encoding := range resp.TransferEncoding {
			if encoding == "chunked" {
				return true
			}
		}
	}

	// 检查Content-Type是否为流媒体类型
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "video/") ||
		strings.Contains(contentType, "audio/") ||
		strings.Contains(contentType, "application/octet-stream") {
		return true
	}

	// 检查自定义头
	if resp.Header.Get("X-Stream") == "true" {
		return true
	}

	// 检查响应Content-Length是否很大（大于10MB）
	contentLength := resp.ContentLength
	if contentLength > 10*1024*1024 {
		return true
	}

	return false
}

// handleStreamResponse 处理流式响应
func handleStreamResponse(w http.ResponseWriter, resp *http.Response) {
	// 写入状态码
	w.WriteHeader(resp.StatusCode)

	// 尝试获取flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Errorf("Streaming not supported")
		io.Copy(w, resp.Body) // 退化为普通复制
		return
	}

	// 使用较大的缓冲区
	buf := make([]byte, 8192)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := w.Write(buf[:n])
			if writeErr != nil {
				log.Errorf("Error writing response data: %v", writeErr)
				return
			}
			flusher.Flush() // 立即将数据发送给客户端
		}

		if err != nil {
			if err != io.EOF {
				log.Warnf("Error reading response data: %v", err)
			}
			break
		}
	}
}

// handleRegularHTTPProxy 处理普通的HTTP请求转发
func handleRegularHTTPProxy(w http.ResponseWriter, r *http.Request, targetIP string) {
	log := log.SugaredLogger()

	// 创建转发请求
	proxyURL := &url.URL{
		Scheme:   "http",
		Host:     targetIP + ":26000",
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// 创建代理请求
	proxyReq, err := http.NewRequest(r.Method, proxyURL.String(), r.Body)
	if err != nil {
		log.Errorf("Failed to create proxy request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 复制原始请求的 header
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 配置支持流式传输的 HTTP 客户端
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// 关键设置: 禁用响应头超时，支持无限期等待数据
		ResponseHeaderTimeout: 0,
	}
	client := &http.Client{
		Transport: transport,
		// 关键设置: 禁用超时，允许长连接和流式传输
		Timeout: 0,
	}

	// 发送代理请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Errorf("Failed to forward request: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// 复制响应 header
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 检查响应，如果是流式内容，使用特殊处理
	if isStreamResponse(resp) {
		log.Infof("Detected streaming response, handling with streaming logic")
		handleStreamResponse(w, resp)
		return
	}

	w.WriteHeader(resp.StatusCode)

	// 使用缓冲复制响应体，提高流式传输性能
	buf := make([]byte, 4096)
	_, err = io.CopyBuffer(w, resp.Body, buf)
	if err != nil && err != io.EOF {
		log.Warnf("Error copying response: %v", err)
	}
}
