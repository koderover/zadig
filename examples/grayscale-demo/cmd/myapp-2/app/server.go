package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"
)

var (
	Count int
	Mutex sync.Mutex
)

// Run launches the app.
func Run(op *Options) error {
	r := gin.Default()

	apiv1 := r.Group("/api/v1")
	{
		apiv1.GET("/ping", func(c *gin.Context) {
			klog.InfoS("Accept request.")

			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})

		apiv1.GET("/info", func(c *gin.Context) {
			klog.InfoS("Accept request.", "headers", c.Request.Header)

			httpMethod := http.MethodGet
			downstreamAddr := fmt.Sprintf("http://%s/api/v1/info", op.DownstreamAddr)

			hasErr := requestDownstream(c, httpMethod, downstreamAddr, op)
			if hasErr {
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "app2",
			})
		})

		apiv1.PUT("/count", func(c *gin.Context) {
			klog.InfoS("Accept request.", "headers", c.Request.Header)

			httpMethod := http.MethodPut
			downstreamAddr := fmt.Sprintf("http://%s/api/v1/count", op.DownstreamAddr)

			hasErr := requestDownstream(c, httpMethod, downstreamAddr, op)
			if hasErr {
				return
			}

			Mutex.Lock()
			Count += 1
			Mutex.Unlock()

			klog.InfoS("app-2 info", "count", Count)

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("app2: %d", Count),
			})
		})

		apiv1.DELETE("/count", func(c *gin.Context) {
			klog.InfoS("Accept request.", "headers", c.Request.Header)

			httpMethod := http.MethodDelete
			downstreamAddr := fmt.Sprintf("http://%s/api/v1/count", op.DownstreamAddr)

			hasErr := requestDownstream(c, httpMethod, downstreamAddr, op)
			if hasErr {
				return
			}

			Mutex.Lock()
			Count = 0
			Mutex.Unlock()

			klog.InfoS("app-2 info", "count", Count)

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("app1: %d", Count),
			})
		})
	}

	return r.Run(op.ListenAddr)
}

func requestDownstream(c *gin.Context, httpMethod string, downstreamAddr string, op *Options) bool {
	req, err := http.NewRequestWithContext(c, httpMethod, downstreamAddr, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})

		return true
	}

	for _, headerKey := range op.Headers {
		headerKey = strings.TrimSpace(headerKey)

		klog.InfoS("Propagate Header", "key", headerKey, "value", c.GetHeader(headerKey))
		req.Header.Set(headerKey, c.GetHeader(headerKey))
	}

	resp, err := op.HttpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})

		return true
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})

		return true
	}
	klog.InfoS("API Result", "result", string(body))
	return false
}
