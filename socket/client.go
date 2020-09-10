package socket

import (
	"fmt"
	"github.com/omzlo/go-sscp"
	"sync"
	"time"
)

const (
	HELLO_MAJOR = 2
	HELLO_MINOR = 0
)

type EventCallback func(*EventConn, *Event) error

type RequestEvent struct {
	Timestamp time.Time
	Callback  EventCallback
	Next      *RequestEvent
	Prev      *RequestEvent
}

/****************************************************************************/

// EventConn
//
//
type EventConn struct {
	Conn           *sscp.Conn
	Addr           string
	ClientName     string
	AuthToken      string
	Connected      bool
	AutoRedial     bool
	Subscriptions  *ChannelSubscriptionList
	MsgId          uint16
	OldestRequest  *RequestEvent
	NewestRequest  *RequestEvent
	Requests       map[uint16]*RequestEvent
	Callbacks      map[EventId]EventCallback
	ProcessError   func(*EventConn, error) bool
	ProcessConnect func(*EventConn) error
	Mutex          sync.Mutex
}

func defaultProcessError(conn *EventConn, err error) bool {
	return false
}
func defaultProcessConnect(conn *EventConn) error {
	return nil
}

func NewEventConn(addr string, client_name string, auth string) *EventConn {
	if addr == "" {
		addr = ":4242"
	}
	return &EventConn{Addr: addr,
		ClientName:     client_name,
		AuthToken:      auth,
		Connected:      false,
		AutoRedial:     true,
		Subscriptions:  NewChannelSubscriptionList(),
		MsgId:          1,
		OldestRequest:  nil,
		NewestRequest:  nil,
		Requests:       make(map[uint16]*RequestEvent),
		Callbacks:      make(map[EventId]EventCallback),
		ProcessError:   defaultProcessError,
		ProcessConnect: defaultProcessConnect}
}

func (conn *EventConn) dial() error {
	sscp_conn, err := sscp.Dial("tcp", conn.Addr, []byte(conn.ClientName), []byte(conn.AuthToken))
	if err != nil {
		return err
	}

	conn.Conn = sscp_conn

	event := NewEvent(conn.MsgId, ClientHelloEvent, NewClientHello(conn.ClientName, HELLO_MAJOR, HELLO_MINOR))
	_, err = event.WriteTo(conn.Conn)
	if err != nil {
		conn.Close()
		return err
	}
	conn.MsgId++

	response := new(Event)
	_, err = response.ReadFrom(conn.Conn)
	if err != nil {
		conn.Close()
		return err
	}

	if response.Id != ServerHelloEvent {
		conn.Close()
		return fmt.Errorf("Expected ServerHelloEvent, got %d (%s)", response.Id, response.Id)
	}

	if response.MsgId != event.MsgId {
		conn.Close()
		return fmt.Errorf("Message Id mismatch on ServerHelloEvent %d != %d", response.MsgId, event.MsgId)
	}

	conn.Connected = true
	return nil
}

func (conn *EventConn) OnEvent(eid EventId, cb EventCallback) {
	if cb == nil {
		delete(conn.Callbacks, eid)
	} else {
		conn.Callbacks[eid] = cb
	}
}

func (conn *EventConn) OnConnect(cb func(*EventConn) error) {
	if cb == nil {
		conn.ProcessConnect = defaultProcessConnect
	} else {
		conn.ProcessConnect = cb
	}
}

func (conn *EventConn) Send(event *Event, cb EventCallback) error {
	conn.Mutex.Lock()
	defer conn.Mutex.Unlock()

	if conn.MsgId == 0 {
		conn.MsgId = 1
	}
	event.MsgId = conn.MsgId
	request := &RequestEvent{Timestamp: time.Now(), Callback: cb, Next: nil}

	conn.Requests[event.MsgId] = request
	if conn.NewestRequest == nil {
		conn.NewestRequest = request
		conn.OldestRequest = request
	} else {
		request.Prev = conn.NewestRequest
		conn.NewestRequest.Next = request
		conn.NewestRequest = request
	}
	conn.MsgId++
	_, err := event.WriteTo(conn.Conn)
	return err
}

func (conn *EventConn) SendAndWaitForResponse(event *Event) *Event {
	echan := make(chan *Event)
	conn.Send(event, func(econn *EventConn, response *Event) error {
		echan <- response
		return nil
	})
	revent := <-echan
	close(echan)
	return revent
}

func (conn *EventConn) Close() error {
	conn.Connected = false
	return conn.Conn.Close()
}

func (conn *EventConn) ListenAndServe() error {
	backoff := 2

	err := conn.dial()
	if err != nil {
		return err
	}

	for {
		if !conn.Connected {
			err = conn.dial()
			if err != nil {
				time.Sleep(time.Duration(backoff) * time.Second)
				if backoff < 1024 {
					backoff *= 2
				}
				continue
			}
			backoff = 3
		}

		event := new(Event)
		_, err := event.ReadFrom(conn.Conn)
		if err != nil {
			conn.Close()
			if !conn.AutoRedial {
				return err
			}
			continue
		}

		if event.MsgId == 0 {
			cb := conn.Callbacks[event.Id]
			if cb != nil {
				if err = cb(conn, event); err != nil {
					if conn.ProcessError(conn, err) {
						return err
					}
				}
			}
		} else {
			req := conn.Requests[event.MsgId]
			if req != nil {
				if err = req.Callback(conn, event); err != nil {
					if conn.ProcessError(conn, err) {
						return err
					}
				}
				conn.Mutex.Lock()
				defer conn.Mutex.Unlock()
				if req == conn.OldestRequest {
					conn.OldestRequest = conn.OldestRequest.Next
					if conn.OldestRequest != nil {
						conn.OldestRequest.Prev = nil
					}
				}
				if req == conn.NewestRequest {
					conn.NewestRequest = conn.NewestRequest.Prev
					if conn.NewestRequest != nil {
						conn.NewestRequest.Next = nil
					}
				}
				delete(conn.Requests, event.MsgId)
			}
		}
	}
	return nil
}
