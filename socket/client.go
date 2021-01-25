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
	processError       func(*EventConn, error) error
	processConnect     func(*EventConn) error
	Mutex              sync.Mutex
	ResponseChannel    chan (Eventer)
	inProcessNextEvent bool
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
		ClientName:      client_name,
		AuthToken:       auth,
		Connected:       false,
		AutoRedial:      false,
		MsgId:           1,
		Callbacks:       make(map[EventId]EventCallback),
		processError:    defaultProcessError,
		processConnect:  defaultProcessConnect,
		ResponseChannel: make(chan (Eventer), 1),
	}
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

	err := conn.processNextEvent()
	if err != nil {
		return err
	}

	timeout := time.NewTimer(3 * time.Second)

	select {
	case response := <-conn.ResponseChannel:
		if !timeout.Stop() {
			<-timeout.C
		}
		ack, ok := response.(*ServerAckEvent)
		if !ok {
			return fmt.Errorf("Expectected ServerAckEvent, got %s", response.Id())
		}

		if response.MsgId() != event.MsgId() {
			return fmt.Errorf("MsgId mismatch is reponse to request %s (request MsgId: %d, response MsgId: %d)", event.Id(), event.MsgId(), response.MsgId())
		}

		conn.MsgId++

		return ack.ToError()
	case <-timeout.C:
		return ErrorServerAckTimeout
	}
}

func (conn *EventConn) Close() error {
	clog.DebugXX("Closing connection to %s", conn.Conn.RemoteAddr())
	conn.Connected = false
	return conn.Conn.Close()
}

func (conn *EventConn) Connect() error {
	return conn.dial()
}

func (conn *EventConn) EnableAutoRedial() *EventConn {
	conn.AutoRedial = true
	return conn
}

func (conn *EventConn) processNextEvent() error {
	// TODO: check if we need atomic change to inProcessNextEvent

	if !conn.inProcessNextEvent {
		conn.inProcessNextEvent = true
		err := conn._processNextEvent()
		conn.inProcessNextEvent = false
		return err
	}
	return nil
}

func (conn *EventConn) _processNextEvent() error {
	for {
		event, err := DecodeEvent(conn.Conn)
		if err != nil {
			conn.Close()
			return err
		}

		if event.MsgId() == 0 {
			cb := conn.Callbacks[event.Id()]
			if cb != nil {
				err := cb(conn, event)
				if err != nil {
					if err := conn.processError(conn, err); err != nil {
						return err
					}
				}
			}
		} else {
			conn.ResponseChannel <- event
			return nil
		}
	}
}

func (conn *EventConn) DispatchEvents() error {
	backoff := 2

	for {
		/* Reconnect automatically if needed. Auto-redial defines the behaviour */
		if !conn.Connected {
			err := conn.dial()
			if err != nil {
				if !conn.AutoRedial {
					return err
				}
				time.Sleep(time.Duration(backoff) * time.Second)
				if backoff < 1024 {
					backoff *= 2
				}
				continue
			}
			backoff = 2
		}

		/* Process events */
		err := conn.processNextEvent()
		if err != nil {
			clog.DebugXX("Event processing returned: %s", err)
			conn.Close()
			if err == Terminate {
				return nil
			}
			if !conn.AutoRedial {
				return err
			}
			continue
		}
	}
}
