package events

import (
	"fmt"
	"io"
	"pannetrat.com/nocand/clog"
	"sync"
	"time"
)

var DefaultManager *EventManager = NewEventManager()

type EventId uint

const (
	StopEvent EventId = iota
	CanFrameRxEvent
	CanFrameTxEvent
	NocanRxMessageEvent
	NocanTxMessageEvent
	NocanPublishEvent
	NocanSysAddressRequestEvent
	NocanSysAddressConfigureEvent
	NocanSysAddressConfigureAckEvent
	NocanSysAddressLookupEvent
	NocanSysAddressLookupAckEvent
	NocanSysNodeBootRequestEvent
	NocanSysNodeBootAckEvent
	NocanSysNodePingEvent
	NocanSysNodePingAckEvent
	NocanSysChannelRegisterEvent
	NocanSysChannelRegisterAckEvent
	NocanSysChannelUnregisterEvent
	NocanSysChannelUnregisterAckEvent
	NocanSysChannelSubscribeEvent
	NocanSysChannelUnsubscribeEvent
	NocanSysChannelLookupEvent
	NocanSysChannelLookupAckEvent
	NocanSysBootloaderGetSignatureEvent
	NocanSysBootloaderGetSignatureAckEvent
	NocanSysBootloaderSetAddressEvent
	NocanSysBootloaderSetAddressAckEvent
	NocanSysBootloaderWriteEvent
	NocanSysBootloaderWriteAckEvent
	NocanSysBootloaderReadEvent
	NocanSysBootloaderReadAckEvent
	NocanSysBootloaderLeaveEvent
	NocanSysBootloaderLeaveAckEvent
	NocanSysBootloaderEraseEvent
	NocanSysBootloaderEraseAckEvent
	NocanSysReservedEvent
	NocanSysDebugMessageEvent
	BusPowerStatusEvent
	ServerAckEvent
	ClientAuthEvent
	ClientSubscribeEvent
	ClientPingEvent
	CountEventIds
)

var EventNames = [CountEventIds]string{
	"stop-event",
	"can-frame-rx-event",
	"can-frame-tx-event",
	"nocan-rx-message-event",
	"nocan-tx-message-event",
	"nocan-publish",
	"nocan-sys-address-request",
	"nocan-sys-address-configure",
	"nocan-sys-address-configure-ack",
	"nocan-sys-address-lookup",
	"nocan-sys-address-lookup-ack",
	"nocan-sys-node-boot-request",
	"nocan-sys-node-boot-ack",
	"nocan-sys-node-ping",
	"nocan-sys-node-ping-ack",
	"nocan-sys-channel-register",
	"nocan-sys-channel-register-ack",
	"nocan-sys-channel-unregister",
	"nocan-sys-channel-unregister-ack",
	"nocan-sys-channel-subscribe",
	"nocan-sys-channel-unsubscribe",
	"nocan-sys-channel-lookup",
	"nocan-sys-channel-lookup-ack",
	"nocan-sys-bootloader-get-signature",
	"nocan-sys-bootloader-get-signature-ack",
	"nocan-sys-bootloader-set-address",
	"nocan-sys-bootloader-set-address-ack",
	"nocan-sys-bootloader-write",
	"nocan-sys-bootloader-write-ack",
	"nocan-sys-bootloader-read",
	"nocan-sys-bootloader-read-ack",
	"nocan-sys-bootloader-leave",
	"nocan-sys-bootloader-leave-ack",
	"nocan-sys-bootloader-erase",
	"nocan-sys-bootloader-erase-ack",
	"nocan-sys-reserved",
	"nocan-sys-debug-message",
	"bus-power-status-event",
	"server-ack-event",
	"client-auth-event",
	"client-subscribe-event",
	"client-ping-event",
}

func (id EventId) String() string {
	if id < CountEventIds {
		return EventNames[id]
	}
	return "!unknown-even-id!"
}

type ValuePacker interface {
	PackValue() ([]byte, error)
}

type ValueUnpackerFunction func(EventId, []byte) (ValuePacker, error)

type EventHandler interface {
	HandleEvent(*Event)
}

type EventHandlerFunctionWrapper struct {
	Function func(*Event)
}

// Event
//
//
type Event struct {
	Id    EventId
	Value ValuePacker
}

func NewEvent(eid EventId, val ValuePacker) *Event {
	return &Event{Id: eid, Value: val}
}

// EmptyValue implements the ValuePacker interface for events with no value
//
//
type EmptyValue struct{}

func NewEmptyEvent(eid EventId) *Event {
	return NewEvent(eid, &EmptyValue{})
}

func (e *EmptyValue) PackValue() ([]byte, error) {
	return nil, nil
}

func DecodeEmptyValue(e EventId, b []byte) (ValuePacker, error) {
	return &EmptyValue{}, nil
}

// EventListener
//
//
type EventListener struct {
	Handler EventHandler
	Next    *EventListener
	Once    bool
}

type EventListenerList struct {
	First *EventListener
	Last  *EventListener
}

type EventManager struct {
	Mutex     sync.RWMutex
	Map       [CountEventIds]EventListenerList
	Queue     chan *Event
	Terminate chan bool
}

var EventDecoders [CountEventIds]ValueUnpackerFunction

func RegisterValueUnpackerFunction(id EventId, fn ValueUnpackerFunction) {
	if EventDecoders[id] != nil {
		panic(fmt.Sprintf("Attempt to register a second ValueUnpackerFunction for event %s (%d)", id, id))
	}
	EventDecoders[id] = fn
}

func EncodeToStream(w io.Writer, e *Event) error {
	pv, err := e.Value.PackValue()
	if err != nil {
		fmt.Errorf("Failed to encode value of event %d (%s), %s", e.Id, e.Id, err)
	}

	buf := PackEvent(e.Id, pv)

	_, err = w.Write(buf)
	if err != nil {
		return fmt.Errorf("Failed to write %d bytes for value of encoded event %d (%s), %s", len(buf), e.Id, e.Id, err)
	}
	return nil
}

func DecodeFromStream(r io.Reader) (*Event, error) {
	var rbuf [4]byte
	var rlen uint
	var err error

	_, err = r.Read(rbuf[:2])
	if err != nil {
		return nil, fmt.Errorf("Begin read failed in event decoding, %s", err)
	}

	eventId := EventId(rbuf[0])

	if eventId >= CountEventIds {
		return nil, fmt.Errorf("Unexpected event id: %d", eventId)
	}

	if rbuf[1] <= 0x80 {
		rlen = uint(rbuf[1])
	} else {
		switch rbuf[1] & 0xF {
		case 1:
			_, err = r.Read(rbuf[:1])
			rlen = uint(rbuf[0])
		case 2:
			_, err = r.Read(rbuf[:2])
			rlen = (uint(rbuf[0]) << 8) | uint(rbuf[1])
		case 3:
			_, err = r.Read(rbuf[:3])
			rlen = (uint(rbuf[0]) << 16) | (uint(rbuf[1]) << 8) | uint(rbuf[2])
		case 4:
			_, err = r.Read(rbuf[:4])
			rlen = (uint(rbuf[0]) << 24) | (uint(rbuf[1]) << 16) | (uint(rbuf[2]) << 8) | uint(rbuf[3])
		default:
			return nil, fmt.Errorf("Wrong byte length in event decoding (got %d)", rbuf[1])
		}
		if err != nil {
			return nil, fmt.Errorf("Length read failed in event decoding, %s", err)
		}
	}

	dbuf := make([]byte, rlen)
	_, err = r.Read(dbuf)
	if err != nil {
		return nil, fmt.Errorf("Read %d bytes failed while decoding value for event %d (%s), %s", rlen, eventId, eventId, err)
	}

	decoder := EventDecoders[eventId]
	if decoder == nil {
		return nil, fmt.Errorf("Failed to find a decoder for event %d (%s)", eventId, eventId)
	}

	eventValue, err := decoder(eventId, dbuf)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode value for event %d (%s), %s", eventId, eventId, err)
	}
	return NewEvent(eventId, eventValue), nil
}

func EventHandlerFunction(fn func(*Event)) *EventHandlerFunctionWrapper {
	return &EventHandlerFunctionWrapper{fn}
}

func (ehfw *EventHandlerFunctionWrapper) HandleEvent(e *Event) {
	ehfw.Function(e)
}

func NewEventManager() *EventManager {
	em := &EventManager{
		Queue:     make(chan *Event, 32),
		Terminate: make(chan bool),
	}
	go em.Run()
	return em
}

func (e *EventManager) addListener(eventId EventId, once bool, handler EventHandler) {
	var listeners EventListenerList

	event_listener := &EventListener{Handler: handler, Next: nil, Once: once}

	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	if listeners = e.Map[eventId]; listeners.First == nil {
		listeners = EventListenerList{First: event_listener, Last: event_listener}
	} else {
		listeners.Last.Next = event_listener
		listeners.Last = event_listener
	}
	e.Map[eventId] = listeners
}

func (e *EventManager) AddListener(eventId EventId, handler EventHandler) {
	e.addListener(eventId, false, handler)
}

func (e *EventManager) On(eventId EventId, handler EventHandler) {
	e.AddListener(eventId, handler)
}

func (e *EventManager) AddOnceListener(eventId EventId, handler EventHandler) {
	e.addListener(eventId, true, handler)
}

func (e *EventManager) Once(eventId EventId, handler EventHandler) {
	e.AddOnceListener(eventId, handler)
}

func (e *EventManager) RemoveListener(eventId EventId, handler EventHandler) bool {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	// Linus would not approve ;-)
	var pre *EventListener
	var cur *EventListener

	pre = nil
	cur = e.Map[eventId].First
	for cur != nil {
		if cur.Handler == handler {

			if pre == nil {
				e.Map[eventId] = EventListenerList{First: cur.Next, Last: e.Map[eventId].Last}
			} else {
				pre.Next = cur.Next
			}

			if cur.Next == nil {
				if pre == nil {
					e.Map[eventId] = EventListenerList{First: nil, Last: nil}
				} else {
					e.Map[eventId] = EventListenerList{First: e.Map[eventId].First, Last: pre}
				}
			}

			cur.Next = nil // avoid memory leak

			return true
		}
		pre = cur
		cur = cur.Next
	}
	return false
}

func (e *EventManager) removeOnceListeners(eventId EventId) {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	// Linus would not approve ;-)
	var pre *EventListener
	var cur *EventListener
	var nex *EventListener

	pre = nil
	cur = e.Map[eventId].First
	for cur != nil {

		nex = cur.Next

		if cur.Once {
			if pre == nil {
				e.Map[eventId] = EventListenerList{First: cur.Next, Last: e.Map[eventId].Last}
			} else {
				pre.Next = cur.Next
			}

			if cur.Next == nil {
				e.Map[eventId] = EventListenerList{First: e.Map[eventId].First, Last: pre}
			}

			cur.Next = nil // avoid memory leak
		}
		pre = cur
		cur = nex
	}
}

func (e *EventManager) Emit(event *Event) {
	e.Queue <- event
}

func (e *EventManager) Run() {

	for {
		event := <-e.Queue

		clog.DebugXX("Got event %s", event.Id)

		do_once := false

		e.Mutex.RLock()
		listeners := e.Map[event.Id]
		for listener := listeners.First; listener != nil; listener = listener.Next {
			//clog.DebugXX("Calling listener %v for event %s", listener.Handler, event.Id)
			listener.Handler.HandleEvent(event)

			if listener.Once {
				do_once = true
			}
		}
		e.Mutex.RUnlock()

		if do_once {
			e.removeOnceListeners(event.Id)
		}

		if event.Id == StopEvent {
			e.Terminate <- true
			break
		}
	}
}

func (e *EventManager) Stop() {
	e.Emit(NewEmptyEvent(StopEvent))
	_ = <-e.Terminate
}

type EventWaiter struct {
	Notification chan *Event
	Timeout      *time.Timer
}

func NewEventWaiter(timeout time.Duration) *EventWaiter {
	ew := new(EventWaiter)
	ew.Notification = make(chan *Event)
	ew.Timeout = time.NewTimer(timeout)
	return ew
}

func (ew *EventWaiter) HandleEvent(event *Event) {
	ew.Notification <- event
}

func (e *EventManager) WaitFor(eventId EventId, timeout time.Duration) (*Event, error) {
	ew := NewEventWaiter(timeout)
	e.AddOnceListener(eventId, ew)

	select {
	case e := <-ew.Notification:
		if !ew.Timeout.Stop() {
			<-ew.Timeout.C
		}
		return e, nil
	case <-ew.Timeout.C:
		e.RemoveListener(eventId, ew)
		return nil, fmt.Errorf("Response timeout (%s)", timeout)
	}
}

/****/

func AddListener(eventId EventId, handler EventHandler) {
	DefaultManager.AddListener(eventId, handler)
}

func On(eventId EventId, handler EventHandler) {
	DefaultManager.AddListener(eventId, handler)
}

func AddOnceListener(eventId EventId, handler EventHandler) {
	DefaultManager.AddOnceListener(eventId, handler)
}

func Once(eventId EventId, handler EventHandler) {
	DefaultManager.AddOnceListener(eventId, handler)
}

func RemoveListener(eventId EventId, handler EventHandler) bool {
	return DefaultManager.RemoveListener(eventId, handler)
}

func Emit(event *Event) {
	DefaultManager.Emit(event)
}

func WaitFor(eventId EventId, timeout time.Duration) (*Event, error) {
	return DefaultManager.WaitFor(eventId, timeout)
}
