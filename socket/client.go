package socket

import (
	"errors"
	"fmt"
	"github.com/omzlo/go-sscp"
	"sync"
	"time"
)

const (
	HELLO_MAJOR = 2
	HELLO_MINOR = 0
)

type RequestEvent struct {
	Timestamp time.Time
	Callback  EventCallback
	Next      *RequestEvent
	Prev      *RequestEvent
}

type EventCallback func(*EventConn, Eventer) error

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
	MsgId          uint16
	Callbacks      map[EventId]EventCallback
	processError   func(*EventConn, error) error
	processConnect func(*EventConn) error
	Mutex          sync.Mutex
}

func defaultProcessError(conn *EventConn, err error) error {
	return err
}
func defaultProcessConnect(conn *EventConn) error {
	return nil
}

var Terminate = errors.New("Client termination request")

func NewEventConn(addr string, client_name string, auth string) *EventConn {
	if addr == "" {
		addr = ":4242"
	}
	return &EventConn{Addr: addr,
		ClientName:     client_name,
		AuthToken:      auth,
		Connected:      false,
		AutoRedial:     true,
		MsgId:          1,
		Callbacks:      make(map[EventId]EventCallback),
		processError:   defaultProcessError,
		processConnect: defaultProcessConnect}
}

func (conn *EventConn) dial() error {
	sscp_conn, err := sscp.Dial("tcp", conn.Addr, []byte(conn.ClientName), []byte(conn.AuthToken))
	if err != nil {
		return err
	}

	conn.Conn = sscp_conn

	event := NewClientHelloEvent(conn.ClientName, HELLO_MAJOR, HELLO_MINOR)
	event.SetMsgId(1)

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

func (conn *EventConn) OnError(cb func(*EventConn, error) error) {
	if cb == nil {
		conn.processError = defaultProcessError
	} else {
		conn.processError = cb
	}
}

func (conn *EventConn) Send(event Eventer) error {
	if conn.MsgId == 0 {
		conn.MsgId = 1
	}
	event.SetMsgId(conn.MsgId)

	if err := EncodeEvent(conn.Conn, event); err != nil {
		return err
	}

	response, err := conn.processNextEvent()
	if err != nil {
		return err
	}

	ack, ok := response.(*ServerAckEvent)
	if !ok {
		return fmt.Errorf("Expectected ServerAckEvent, got %s", response.Id())
	}

	if response.MsgId() != event.MsgId() {
		return fmt.Errorf("MsgId mismatch is reponse to request %s (request MsgId: %d, response MsgId: %d)", event.Id(), event.MsgId(), response.MsgId())
	}

	return ack.ToError()
}

func (conn *EventConn) Close() error {
	conn.Connected = false
	return conn.Conn.Close()
}

func (conn *EventConn) Connect() error {
	return conn.dial()
}

func (conn *EventConn) processNextEvent() (Eventer, error) {
	for {
		event, err := DecodeEvent(conn.Conn)
		if err != nil {
			conn.Close()
			return nil, err
		}

		if event.MsgId() == 0 {
			cb := conn.Callbacks[event.Id()]
			if cb != nil {
				err := cb(conn, event)
				if err != nil {
					if err := conn.processError(conn, err); err != nil {
						return nil, err
					}
				}
			}
		} else {
			return event, err
		}
	}
}

func (conn *EventConn) DispatchEvents() error {
	backoff := 2

	/* First connect, if Connect() was not called previously */
	if !conn.Connected {
		err := conn.dial()
		if err != nil {
			return err
		}
	}

	for {
		/* Reconnect automatically if needed. Auto-redial will define the behaviour */
		if !conn.Connected {
			err := conn.dial()
			if err != nil {
				time.Sleep(time.Duration(backoff) * time.Second)
				if backoff < 1024 {
					backoff *= 2
				}
				continue
			}
			backoff = 3
		}

		/* Process events */
		event, err := conn.processNextEvent()
		if err != nil {
			conn.Close()
			if err == Terminate {
				return nil
			}
			if !conn.AutoRedial {
				return err
			}
			continue
		}

		return fmt.Errorf("Unexpected event %s", event.Id())
	}
}
