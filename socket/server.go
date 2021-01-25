package socket

import (
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/go-sscp"
	"io"
	"sync"
)

/****************************************************************************/

// ClientDescriptor represents a single connection from an external client through TCP/IP
//
//
type ClientDescriptor struct {
	Id              uint
	Server          *Server
	Conn            *sscp.Conn
	OutputChan      chan Eventer
	TerminationChan chan struct{}
	ChannelFilter   *ChannelFilterEvent
	Connected       bool
	Next            *ClientDescriptor
	LastMsgId       uint16
}

func (c *ClientDescriptor) Name() string {
	return fmt.Sprintf("%d (%s)", c.Id, c.Conn.RemoteAddr())
}

func (c *ClientDescriptor) SendEvent(event Eventer) error {
	if !c.Connected {
		return fmt.Errorf("SendEvent failed, client %d is not connected", c.Id)
	}
	c.OutputChan <- event
	return nil
}

func (c *ClientDescriptor) SendAck(ack byte) error {
	response := NewServerAckEvent(ack)
	response.SetMsgId(c.LastMsgId)
	return c.SendEvent(response)
}

func clientChannelFilterHandler(c *ClientDescriptor, event Eventer) error {
	sl := event.(*ChannelFilterEvent)

	c.ChannelFilter = sl

	return c.SendAck(ServerAckSuccess)
}

/****************************************************************************/

// Server
//
//

type EventHandler func(*ClientDescriptor, Eventer) error

type Server struct {
	Mutex     sync.Mutex
	AuthToken string
	topId     uint
	ls        *sscp.Listener
	clients   *ClientDescriptor
	handlers  map[EventId]EventHandler
}

func NewServer() *Server {
	s := &Server{handlers: make(map[EventId]EventHandler)}
	s.RegisterHandler(ChannelFilterEventId, clientChannelFilterHandler)
	return s
}

func (s *Server) NewClient(conn *sscp.Conn) *ClientDescriptor {
	c := new(ClientDescriptor)
	c.ChannelFilter = nil
	c.Server = s
	c.Conn = conn

	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	c.Next = s.clients
	c.OutputChan = make(chan Eventer, 16)
	c.TerminationChan = make(chan struct{})
	c.Connected = true

	c.Id = s.topId
	s.topId++
	s.clients = c
	return c
}

func (s *Server) DeleteClient(c *ClientDescriptor) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	c.Connected = false
	c.Conn.Close()

	close(c.OutputChan)
	close(c.TerminationChan)

	ptr := &s.clients
	for *ptr != nil {
		if *ptr == c {
			*ptr = c.Next
			clog.DebugXX("Deleting client %s, closing channel and socket", c.Name())
			return true
		}
		ptr = &((*ptr).Next)
	}
	clog.Error("Internal error: failed to delete client %s", c.Name())
	return false
}

func (s *Server) Broadcast(event Eventer, exclude_client *ClientDescriptor) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	for c := s.clients; c != nil; c = c.Next {
		if c == exclude_client {
			continue
		}
		if event.Id() == ChannelUpdateEventId {
			channel_update := event.(*ChannelUpdateEvent)
			if c.ChannelFilter == nil || c.ChannelFilter.Includes(channel_update.ChannelId) {
				c.SendEvent(event)
			}
		} else {
			c.SendEvent(event)
		}
	}
}

func (s *Server) RegisterHandler(eid EventId, fn EventHandler) {
	if s.handlers[eid] != nil {
		clog.Warning("Replacing existing event handler for event %d", eid)
	}
	s.handlers[eid] = fn
}

func dumpValue(value []byte) string {
	if len(value) > 64 {
		return fmt.Sprintf("%q (%d additional bytes hidden)", value[:64], len(value)-64)
	}
	return fmt.Sprintf("%q", value)
}

func (s *Server) runClient(c *ClientDescriptor) {
	/* Step 1: Decode client-hello-event */
	e, err := DecodeEvent(c.Conn)
	if err != nil {
		clog.Warning("Could not decode client-hello-event: %s", err)
		c.SendAck(ServerAckBadRequest)
		return
	}

	client_hello, ok := e.(*ClientHelloEvent)
	if !ok {
		clog.Warning("Expected client-hello-event got %s instead.", e.Id())
		c.SendAck(ServerAckBadRequest)
		return
	}

	/* Step 2: Send server-hello-event */
	server_hello := NewServerHelloEvent("nocand", 0, 0)
	server_hello.SetMsgId(client_hello.MsgId())
	if err := EncodeEvent(c.Conn, server_hello); err != nil {
		clog.Warning("Could not encode server-hello-event: %s", err)
		c.SendAck(ServerAckBadRequest)
		return
	}
	c.LastMsgId = client_hello.MsgId()

	/* Step 3: Run client sending process. */
	go func() {
		defer s.DeleteClient(c)
		for {
			select {
			case event := <-c.OutputChan:
				if err := EncodeEvent(c.Conn, event); err != nil {
					clog.Warning("Client %s: %s", c.Name(), err)
					c.Conn.Close()
					// Wait for termination and exit the goroutine
					<-c.TerminationChan
					return
				}
			case <-c.TerminationChan:
				return
			}
		}
	}()

	/* Step 4: Run client receiving process. */
	for {
		event, err := DecodeEvent(c.Conn)

		if err != nil {
			if err != io.EOF {
				clog.Warning("Message reception error for client %s: %s", c.Name(), err)
				c.LastMsgId++ // blindly assume this
				c.SendAck(ServerAckBadRequest)
			}
			break
		}

		clog.DebugX("Processing event %s(%d) from client %s", event.Id(), event.Id(), c.Name())

		if event.MsgId() != 0 {
			c.LastMsgId++
			if c.LastMsgId == 0 {
				c.LastMsgId = 1
			}
			if c.LastMsgId != event.MsgId() {
				clog.Warning("Client MsgId mismatch: expected %d but got %d", c.LastMsgId, event.MsgId())
				c.SendAck(ServerAckBadRequest)
				break
			}
		}

		handler := s.handlers[event.Id()]
		if handler != nil {
			if err = handler(c, event); err != nil {
				clog.Error("Handler for event %s(%d) failed: %s", event.Id(), event.Id(), err)
				// SendAck is performed by handler() here
				break
			}
		} else {
			clog.Warning("No handler found for event id %d", event.Id())
			c.LastMsgId++ // blindly assume this
			c.SendAck(ServerAckBadRequest)
			break
		}
	}
	c.TerminationChan <- struct{}{}
	/* This will result in a call to s.DeleteClient(c) and the termination of the
	   goroutine launched at step 3.
	   s.DeleteClient closes the socket and channels associated to the client.
	   It also prints a debug message confirming the deletion of the client.
	*/
}

func (s *Server) ListenAndServe(addr string, auth_token string) error {
	ls, err := sscp.Listen("tcp", addr, []byte("nocand"), []byte(auth_token))
	if err != nil {
		return err
	}

	clog.Info("Listening for clients at %s", ls.Addr())
	s.ls = ls

	go func() {
		for {
			conn, err := s.ls.Accept()
			if err != nil {
				clog.Error("Server could not accept connection: %s", err)
			} else {

				clog.Debug("Created and authenticated new client %s", conn.RemoteAddr())
				client := s.NewClient(conn)
				go s.runClient(client)
			}
		}
	}()
	return nil
}
