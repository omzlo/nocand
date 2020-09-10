package socket

import (
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/go-sscp"
	"github.com/omzlo/nocand/models/nocan"
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
	OutputChan      chan *Event
	TerminationChan chan bool
	Subscriptions   *ChannelSubscriptionList
	Connected       bool
	Next            *ClientDescriptor
}

func (c *ClientDescriptor) Name() string {
	return fmt.Sprintf("%d (%s)", c.Id, c.Conn.RemoteAddr())
}

func (c *ClientDescriptor) SendEvent(event *Event) error {
	if !c.Connected {
		return fmt.Errorf("Put failed, client %s is not connected", c)
	}
	c.OutputChan <- event
	return nil
}

func (c *ClientDescriptor) SendAsEvent(eid EventId, value Packer) error {
	return c.SendEvent(NewEvent(0, eid, value))
}

func (c *ClientDescriptor) RespondToEvent(event *Event, eid EventId, value Packer) error {
	e := NewEvent(event.MsgId, eid, value)
	return c.SendEvent(e)
}

/*
func (c *ClientDescriptor) ReceiveEvent() (*Event, error) {
	if !c.Connected {
		return nil, fmt.Errorf("Get failed, client %s is not connected", c)
	}

	event := new(Event)
	_, err := event.ReadFrom(c.Conn)
	if err != nil {
		return nil, err
	}
	return event, nil
}
*/

func clientSubscribeHandler(c *ClientDescriptor, event *Event) error {
	sl := event.Value.(*ChannelSubscriptionList)

	c.Subscriptions = sl

	return c.RespondToEvent(event, ServerAckEvent, ServerAckSuccess)
}

/****************************************************************************/

// Server
//
//

type EventHandler func(*ClientDescriptor, *Event) error

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
	s.RegisterHandler(ChannelSubscribeEvent, clientSubscribeHandler)
	return s
}

func (s *Server) NewClient(conn *sscp.Conn) *ClientDescriptor {
	c := new(ClientDescriptor)
	c.Subscriptions = NewChannelSubscriptionList()
	c.Server = s
	c.Conn = conn
	// TODO: move this next line after mutex.lock
	c.Next = s.clients
	c.OutputChan = make(chan *Event, 16)
	c.TerminationChan = make(chan bool)
	c.Connected = true

	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	c.Id = s.topId
	s.topId++
	s.clients = c
	return c
}

func (s *Server) DeleteClient(c *ClientDescriptor) bool {
	c.Connected = false
	c.Conn.Close()
	//close(c.OutputChan)

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	ptr := &s.clients
	for *ptr != nil {
		if *ptr == c {
			*ptr = c.Next
			clog.DebugXX("Deleting client %s, closing channel and socket", c)
			return true
		}
		ptr = &((*ptr).Next)
	}
	return false
}

func (s *Server) Broadcast(eid EventId, value Packer, exclude_client *ClientDescriptor) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	event := NewEvent(0, eid, value)

	for c := s.clients; c != nil; c = c.Next {
		if c == exclude_client {
			continue
		}
		if event.Id == ChannelUpdateEvent {
			channel_update := event.Value.(*ChannelUpdate)
			if c.Subscriptions.Includes(channel_update.Id) || c.Subscriptions.Includes(nocan.UNDEFINED_CHANNEL) {
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
	go func() {
		for {
			select {
			case event := <-c.OutputChan:
				if _, err := event.WriteTo(c.Conn); err != nil {
					clog.Warning("Client %s: %s", c, err)
					c.TerminationChan <- true
				}
			case <-c.TerminationChan:
				s.DeleteClient(c)
				return
			}
		}
	}()

	for {
		event := new(Event)
		_, err := event.ReadFrom(c.Conn)

		if err != nil {
			if err == io.EOF {
				clog.Info("Client %s closed connection", c)
			} else {
				clog.Warning("Client %s: %s", c, err)
			}
			break
		}

		clog.DebugX("Processing event %s(%d) from client %s", event.Id, event.Id, c)

		handler := s.handlers[event.Id]
		if handler != nil {
			if err = handler(c, event); err != nil {
				clog.Error("Handler for event %s(%d) failed: %s", event.Id, event.Id, err)
				break
			}
		} else {
			clog.Warning("No handler found for event id %d", event.Id)
			break
		}
	}
	c.TerminationChan <- true
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
				client := s.NewClient(conn)
				clog.Debug("Created and authenticated new client %s", client)
				go s.runClient(client)
			}
		}
	}()
	return nil
}
