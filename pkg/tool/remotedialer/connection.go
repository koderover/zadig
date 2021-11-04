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
	"io"
	"net"
	"time"
)

type connection struct {
	err           error
	writeDeadline time.Time
	buffer        *readBuffer
	addr          addr
	session       *Session
	connID        int64
}

func newConnection(connID int64, session *Session, proto, address string) *connection {
	c := &connection{
		buffer: newReadBuffer(),
		addr: addr{
			proto:   proto,
			address: address,
		},
		connID:  connID,
		session: session,
	}
	return c
}

func (c *connection) tunnelClose(err error) {
	c.writeErr(err)
	c.doTunnelClose(err)
}

func (c *connection) doTunnelClose(err error) {
	if c.err != nil {
		return
	}

	c.err = err
	if c.err == nil {
		c.err = io.ErrClosedPipe
	}

	c.buffer.Close(c.err)
}

func (c *connection) OnData(m *message) error {
	return c.buffer.Offer(m.body)
}

func (c *connection) Close() error {
	c.session.closeConnection(c.connID, io.EOF)
	return nil
}

func (c *connection) Read(b []byte) (int, error) {
	n, err := c.buffer.Read(b)
	return n, err
}

func (c *connection) Write(b []byte) (int, error) {
	if c.err != nil {
		return 0, io.ErrClosedPipe
	}
	msg := newMessage(c.connID, b)
	return c.session.writeMessage(c.writeDeadline, msg)
}

func (c *connection) writeErr(err error) {
	if err != nil {
		msg := newErrorMessage(c.connID, err)
		c.session.writeMessage(c.writeDeadline, msg)
	}
}

func (c *connection) LocalAddr() net.Addr {
	return c.addr
}

func (c *connection) RemoteAddr() net.Addr {
	return c.addr
}

func (c *connection) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *connection) SetReadDeadline(t time.Time) error {
	c.buffer.deadline = t
	return nil
}

func (c *connection) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

type addr struct {
	proto   string
	address string
}

func (a addr) Network() string {
	return a.proto
}

func (a addr) String() string {
	return a.address
}
