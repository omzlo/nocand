package socket

import (
	"errors"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/go-sscp"
	"sync"
	"time"
)

const (
	HELLO_MAJOR = 2
	HELLO_MINOR = 0
)

type EventCallback func(*EventConn, Eventer) error

/****************************************************************************/

type EventRequest struct {
	ResponseCallback func(*EventConn, error) error
	CreatedAt        time.Time
}

// EventConn
//
//
type EventConn struct {
	Conn               *sscp.Conn
	Addr               string
	ClientName         string
	AuthToken          string
	Connected          bool
	AutoRedial         bool
	MsgId              uint16
	Callbacks          map[EventId]EventCallback
	processConnect     func(*EventConn) error
	Mutex              sync.Mutex
	pendingRequests    map[uint16]*EventRequest
	endOfReadChannel   chan error
	terminationChannel chan error
	eventChannel       chan Eventer
	dialCount          int
}

/* Some key logic in EventConn:

1) There is only one goroutine that reads messages from an EventConn.
   - This is either the dial() function or the conn.readEventLoop() goroutine called from dial()
   - dial() may be called several times if auto-redial is enabled.
   - However, there is always a call to Close() between calls to dial()
   - Close() always waits for conn.readEventLoop() to end.
   - Hence there are never 2 conn.readEventLoop() goroutines running in parallel.

2) There is only one goroutine that can writes messages to an EventConn at a time.
   - dial() and SendAsync() are the only functions that write to an EventConn.
   - SendAsync() uses a mutex to avoid concurrent writes to a conn
   - SendAsync() does not allow writes when conn.Connect is false
   - dial() will perform a write when conn.Connect is false
   - Hence dial() and SendAsync() do not overlap

*/

func defaultProcessConnect(conn *EventConn) error {
	return nil
}

var Terminate = errors.New("Client termination request")
var TimeoutError = errors.New("Event response timeout")

func NewEventConn(addr string, client_name string, auth string) *EventConn {
	if addr == "" {
		addr = ":4242"
	}
	return &EventConn{Addr: addr,
		ClientName:         client_name,
		AuthToken:          auth,
		Connected:          false,
		AutoRedial:         false,
		MsgId:              1,
		Callbacks:          make(map[EventId]EventCallback),
		processConnect:     defaultProcessConnect,
		pendingRequests:    make(map[uint16]*EventRequest),
		endOfReadChannel:   make(chan error, 1),
		terminationChannel: make(chan error, 1),
		eventChannel:       make(chan Eventer, 32),
		dialCount:          0,
	}
}

func (conn *EventConn) dial() error {
	sscp_conn, err := sscp.Dial("tcp", conn.Addr, []byte(conn.ClientName), []byte(conn.AuthToken))
	if err != nil {
		return err
	}

	conn.Conn = sscp_conn
	conn.MsgId = 1

	event := NewClientHelloEvent(conn.ClientName, HELLO_MAJOR, HELLO_MINOR)
	event.SetMsgId(conn.MsgId)

	if err = EncodeEvent(conn.Conn, event); err != nil {
		conn.Close()
		return err
	}
	conn.MsgId++

	response, err := DecodeEvent(conn.Conn)
	if err != nil {
		conn.Close()
		return err
	}

	if response.Id() != ServerHelloEventId {
		conn.Close()
		return fmt.Errorf("Expected ServerHelloEvent, got %d (%s)", response.Id(), response.Id())
	}

	if response.MsgId() != event.MsgId() {
		conn.Close()
		return fmt.Errorf("Message Id mismatch on ServerHelloEvent %d != %d", response.MsgId(), event.MsgId())
	}

	conn.Connected = true

	go conn.readEventLoop()

	conn.dialCount++
	if conn.dialCount == 1 {
		go conn.processEventLoop()
	}

	clog.Debug("Opened connection %d to %s from %s", conn.dialCount, conn.Conn.RemoteAddr(), conn.Conn.LocalAddr())
	return conn.processConnect(conn)
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
		conn.processConnect = defaultProcessConnect
	} else {
		conn.processConnect = cb
	}
}

func (conn *EventConn) SendAsync(request Eventer, cb func(*EventConn, error) error) {
	conn.Mutex.Lock()
	defer conn.Mutex.Unlock()

	if !conn.Connected {
		err := errors.New("Sending event on a closed connection.")
		cb(conn, err)
		return
	}

	if conn.MsgId == 0 {
		conn.MsgId = 1
	}
	request.SetMsgId(conn.MsgId)

	conn.pendingRequests[conn.MsgId] = &EventRequest{ResponseCallback: cb, CreatedAt: time.Now()}

	if err := EncodeEvent(conn.Conn, request); err != nil {
		delete(conn.pendingRequests, request.MsgId())
		cb(conn, err)
		return
	}

	conn.MsgId++
}

func (conn *EventConn) Send(request Eventer) error {
	msg_id := request.MsgId()

	conn.SendAsync(request, func(econn *EventConn, ee error) error {
		return ee
	})

	for {
		e_id, err := conn.processNextEvent()
		if e_id == msg_id || err != nil {
			return err
		}
	}
}

func (conn *EventConn) WaitTermination(duration time.Duration) error {
	if duration == 0 {
		err := <-conn.terminationChannel
		if err != Terminate {
			return err
		}
		return nil
	}

	timeout := time.NewTimer(duration)

	select {
	case response := <-conn.terminationChannel:
		if !timeout.Stop() {
			<-timeout.C
		}
		if response != Terminate {
			return response
		}
		return nil
	case <-timeout.C:
		return TimeoutError
	}
}

func (conn *EventConn) Terminate() error {
	conn.AutoRedial = false
	return conn.Close()
}

func (conn *EventConn) Close() error {
	// TODO: clean pending requests
	//clog.DebugXX("Closing connection %d to %s", conn.dialCount, conn.Conn.RemoteAddr())
	conn.Connected = false
	conn.Conn.Close()
	// Wait for read-side close
	select {
	case res := <-conn.endOfReadChannel:
		clog.Debug("Closed connection %d to %s: %s", conn.dialCount, conn.Conn.RemoteAddr(), res)
		return res
	case <-time.After(5 * time.Second):
		clog.Error("Stalled while waiting for connection %d to close on read side. Please report this error.", conn.dialCount)
		return fmt.Errorf("Stalled while waiting for connection to close on the read side")
	}
}

func (conn *EventConn) Connect() error {
	return conn.dial()
}

func (conn *EventConn) EnableAutoRedial() *EventConn {
	conn.AutoRedial = true
	return conn
}

/*
func (conn *EventConn) cancelRequest(request Eventer) {
	er := conn.pendingRequests[request.MsgId()]
	if er != nil {
	}
}
*/

func (conn *EventConn) readEventLoop() {
	for {
		event, err := DecodeEvent(conn.Conn)
		if err != nil {
			conn.endOfReadChannel <- err
			return
		}
		conn.eventChannel <- event
	}
}

func (conn *EventConn) processNextEvent() (uint16, error) {
	event, more := <-conn.eventChannel
	if !more {
		return 0, nil
	}

	msg_id := event.MsgId()

	if msg_id == 0 {
		cb := conn.Callbacks[event.Id()]
		if cb != nil {
			err := cb(conn, event)
			if err != nil {
				return 0, err
			}
		}
		return 0, nil
	}

	conn.Mutex.Lock()
	er := conn.pendingRequests[msg_id]
	if er != nil {
		delete(conn.pendingRequests, msg_id)
	}
	conn.Mutex.Unlock()

	if er == nil {
		return msg_id, fmt.Errorf("Unexpected sequence number %d in response event", msg_id)
	}
	if err := er.ResponseCallback(conn, nil); err != nil {
		return msg_id, err
	}

	conn.Mutex.Lock()
	var expired_id uint16 = 0
	var expired_er *EventRequest = nil
	for event_id, er := range conn.pendingRequests {
		if time.Since(er.CreatedAt) > 3*time.Second {
			delete(conn.pendingRequests, event_id)
			expired_er = er
			expired_id = event_id
			break
		}
	}
	conn.Mutex.Unlock()

	if expired_er != nil {
		if err := er.ResponseCallback(conn, TimeoutError); err != nil {
			return expired_id, err
		}
	}
	return 0, nil
}

func (conn *EventConn) processEventLoop() {
	var err error
	backoff := 1

	for {
		/* Reconnect automatically if needed. Auto-redial defines the behaviour */
		if !conn.Connected {
			err = conn.dial()
			if err != nil {
				if !conn.AutoRedial {
					break
				}
				time.Sleep(time.Duration(backoff) * time.Second)
				if backoff < 8 {
					backoff *= 2
				}
				continue
			}
			backoff = 1
		}

		/* Process events */
		_, err = conn.processNextEvent()
		if err != nil {
			conn.Close()
			if err == Terminate {
				break
			}
			if !conn.AutoRedial {
				break
			}
			continue
		}
	}
	clog.Debug("Ended event loop")
	conn.terminationChannel <- err
}

func ReturnErrorOrContinue(c *EventConn, e error) error {
	return e
}

func ReturnErrorOrTerminate(c *EventConn, e error) error {
	if e == nil {
		return Terminate
	}
	return e
}
