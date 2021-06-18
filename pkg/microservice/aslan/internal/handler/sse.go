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

package handler

import (
	"context"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type producer func(context.Context, chan interface{})

func Stream(c *gin.Context, p producer, log *zap.SugaredLogger) {
	var wg sync.WaitGroup
	streamChan := make(chan interface{}, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-c.Writer.CloseNotify()
		log.Infof("Connection closed, stopping container Log SSE...")
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
