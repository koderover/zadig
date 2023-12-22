package app

import (
	"fmt"
	"net/http"
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

			c.JSON(http.StatusOK, gin.H{
				"message": "app3",
			})
		})

		apiv1.PUT("/count", func(c *gin.Context) {
			klog.InfoS("Accept request.", "headers", c.Request.Header)

			Mutex.Lock()
			Count += 1
			Mutex.Unlock()

			klog.InfoS("app-3 info", "count", Count)

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("app3: %d", Count),
			})
		})

		apiv1.DELETE("/count", func(c *gin.Context) {
			klog.InfoS("Accept request.", "headers", c.Request.Header)

			Mutex.Lock()
			Count = 0
			Mutex.Unlock()

			klog.InfoS("app-3 info", "count", Count)

			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("app1: %d", Count),
			})
		})
	}

	return r.Run(op.ListenAddr)
}
