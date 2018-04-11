package socket

import (
	"fmt"
)

/****************************************************************************/

// Authenticator
//
//
type Authenticator string

func NewClientAuthEvent(auth string) *Event {
	return NewEvent(ClientAuthEvent, Authenticator(auth))
}

func (a Authenticator) PackValue() ([]byte, error) {
	return []byte(a), nil
}

func DecodeAuthenticator(eid EventId, b []byte) (ValuePacker, error) {
	return Authenticator(b), nil
}

/****************************************************************************/

// ServerAck
//
//
type ServerAck byte

const (
	SERVER_SUCCESS ServerAck = iota
	SERVER_BAD_REQUEST
	SERVER_UNAUTHORIZED
	SERVER_NOT_FOUND
	SERVER_GENERAL_FAILURE
)

func NewServerAckEvent(v ServerAck) *Event {
	return NewEvent(ServerAckEvent, v)
}

func (sa ServerAck) PackValue() ([]byte, error) {
	r := make([]byte, 1)
	r[0] = byte(sa)
	return r, nil
}

func DecodeServerAck(eid EventId, b []byte) (ValuePacker, error) {
	if len(b) < 1 {
		return nil, fmt.Errorf("Missing server ack data in encoding")
	}
	return ServerAck(b[0]), nil
}

/****************************************************************************/

// SubscriptionList represents the list of events that a client will receive.
// It also serves as the value of the ClientSubscriptionEvent, so it implements
// the ValuePacker interface
//
type SubscriptionList struct {
	Items map[EventId]bool
}

func NewSubscriptionList(subs ...EventId) *SubscriptionList {
	slist := &SubscriptionList{Items: make(map[EventId]bool)}
	for _, e := range subs {
		slist.Add(e)
	}
	return slist
}

func (sl *SubscriptionList) Add(id EventId) {
	sl.Items[id] = true
}

func (sl *SubscriptionList) Remove(id EventId) {
	delete(sl.Items, id)
}

func (sl *SubscriptionList) Includes(id EventId) bool {
	return sl.Items[id]
}

func (sl *SubscriptionList) PackValue() ([]byte, error) {
	p := make([]byte, len(sl.Items))

	i := 0
	for k, _ := range sl.Items {
		p[i] = byte(k)
		i++
	}
	return p, nil
}

func DecodeSubscriptionList(eid EventId, b []byte) (ValuePacker, error) {
	sl := NewSubscriptionList()
	for _, x := range b {
		sl.Add(EventId(x))
	}
	return sl, nil
}
