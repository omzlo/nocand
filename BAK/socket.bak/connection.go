package socket

import (
	"fmt"
	"net"
	"pannetrat.com/nocand/clog"
	"sync"
)

/****************************************************************************/

// Client represents a single connection from an external client through TCP/IP
//
//
type Client struct {
	Id            uint
	Server        *Server
	Conn          net.Conn
	OutputChan    chan *Event
	Subscriptions *SubscriptionList
	Authenticated bool
	Next          *Client
}

func (s *Server) NewClient(conn net.Conn) *Client {
	c := new(Client)
	c.Subscriptions = NewSubscriptionList()
	c.Server = s
	c.Conn = conn
	c.Next = s.clients
	c.OutputChan = make(chan *Event, 16)

	s.Mutex.Lock()
	c.Id = s.topId
	s.topId++
	s.clients = c
	s.Mutex.Unlock()
	return c
}

func (c *Client) String() string {
	return fmt.Sprintf("%d (%s)", c.Id, c.Conn.RemoteAddr())
}

func (c *Client) Close() bool {
	c.Conn.Close()
	return c.Server.RemoveClient(c)
}

func (c *Client) Put(e *Event) error {
	if err := EncodeToStream(c.Conn, e); err != nil {
		clog.Warning("Client %s: %s", c, err)
		c.Close()
		return err
	}
	return nil
}

func (c *Client) Get() (*Event, error) {
	e, err := DecodeFromStream(c.Conn)
	if err != nil {
		clog.Warning("Client %s: %s", c, err)
		c.Close()
		return nil, err
	}
	return e, nil
}

/****************************************************************************/

// Server
//
//
type Server struct {
	Mutex   sync.Mutex
	topId   uint
	ls      net.Listener
	clients *Client
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) RemoveClient(c *Client) bool {
	c.Conn.Close()

	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	ptr := &s.clients
	for *ptr != nil {
		if *ptr == c {
			*ptr = c.Next
			clog.Info("Deleting client %s", c)
			return true
		}
		ptr = &((*ptr).Next)
	}
	return false
}

func (s *Server) Broadcast(event *Event) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	for c := s.clients; c != nil; c = c.Next {
		if c.Subscriptions.Includes(event.Id) {
			c.OutputChan <- event
		}
	}
}

func (s *Server) Listen(addr string) error {
	ls, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	clog.Info("Listening for clients at %s", ls.Addr())
	s.ls = ls
	return nil
}

func (s *Server) Accept() (*Client, error) {
	conn, err := s.ls.Accept()
	if err != nil {
		return nil, err
	}
	client := s.NewClient(conn)
	clog.Info("Created new client %s", client)
	return client, nil
}

/****************************************************************************/

// EventConn
//
//
type EventConn struct {
	Conn net.Conn
}

func Dial(addr string, auth string) (*EventConn, error) {
	if addr == "" {
		addr = ":4242"
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	ec := &EventConn{Conn: conn}
	if err = ec.Put(NewClientAuthEvent(auth)); err != nil {
		return nil, err
	}
	event, err := ec.Get()
	if err != nil {
		return nil, err
	}
	ack := event.Value.(ServerAck)
	if ack != SERVER_SUCCESS {
		return nil, fmt.Errorf("Authentication failed, code %d", ack)
	}

	return ec, nil
}

func (conn *EventConn) Subscribe(subs *SubscriptionList) error {
	if err := conn.Put(NewEvent(ClientSubscribeEvent, subs)); err != nil {
		return err
	}
	resp, err := conn.Get()
	if err == nil {
		return err
	}
	ack := resp.Value.(ServerAck)
	if ack != SERVER_SUCCESS {
		return fmt.Errorf("Subscription failed, code %d", ack)
	}
	return nil
}

func (conn *EventConn) Get() (*Event, error) {
	return DecodeFromStream(conn.Conn)
}

func (conn *EventConn) Put(event *Event) error {
	err := EncodeToStream(conn.Conn, event)
	if err != nil {
		return err
	}
	return nil
}

func (conn *EventConn) Close() error {
	return conn.Close()
}
