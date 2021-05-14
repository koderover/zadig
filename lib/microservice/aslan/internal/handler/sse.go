package handler

import (
	"context"
	"io"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/koderover/zadig/lib/tool/xlog"
)

type producer func(context.Context, chan interface{})

func Stream(c *gin.Context, p producer, log *xlog.Logger) {
	var wg sync.WaitGroup
	streamChan := make(chan interface{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-c.Writer.CloseNotify()
		log.Infof("Connection closed, Get Container Log SSE stopped")
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		p(ctx, streamChan)
		close(streamChan)
	}()

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-streamChan; ok {
			c.SSEvent("message", msg)
			return true
		}
		return false
	})

	wg.Wait()
}
