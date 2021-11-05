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
	"bytes"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	MaxBuffer = 1 << 20
)

type readBuffer struct {
	cond     sync.Cond
	deadline time.Time
	buf      bytes.Buffer
	err      error
}

func newReadBuffer() *readBuffer {
	return &readBuffer{
		cond: sync.Cond{
			L: &sync.Mutex{},
		},
	}
}

func (r *readBuffer) Offer(reader io.Reader) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	for {
		if r.err != nil {
			return r.err
		}

		if n, err := io.Copy(&r.buf, reader); err != nil {
			return err
		} else if n > 0 {
			r.cond.Broadcast()
		}

		if r.buf.Len() < MaxBuffer {
			return nil
		}

		r.cond.Wait()
	}
}

func (r *readBuffer) Read(b []byte) (int, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	for {
		var (
			n   int
			err error
		)

		if r.buf.Len() > 0 {
			n, err = r.buf.Read(b)
			r.cond.Broadcast()
			if err != io.EOF {
				return n, err
			}
			// buffer remains to be read
			if r.err == io.EOF && r.buf.Len() > 0 {
				return n, nil
			}
			return n, r.err
		}

		if r.err != nil {
			return n, r.err
		}

		now := time.Now()
		if !r.deadline.IsZero() {
			if now.After(r.deadline) {
				return 0, errors.New("deadline exceeded")
			}
		}

		var t *time.Timer
		if !r.deadline.IsZero() {
			t = time.AfterFunc(r.deadline.Sub(now), func() { r.cond.Broadcast() })
		}
		r.cond.Wait()
		if t != nil {
			t.Stop()
		}
	}
}

func (r *readBuffer) Close(err error) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()
	if r.err == nil {
		r.err = err
	}
	r.cond.Broadcast()
	return nil
}
