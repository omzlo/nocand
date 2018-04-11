package socket

import (
	"fmt"
	"io"
)

type EventId byte

const (
	NoEvent EventId = iota
	BusPowerStatusEvent
	ServerAckEvent
	ClientAuthEvent
	ClientSubscribeEvent
	ClientPingEvent
	ChannelUpdateEvent
	CountEventIds
)

var EventNames = [CountEventIds]string{
	"no-event",
	"bus-power-status-event",
	"server-ack-event",
	"client-auth-event",
	"client-subscribe-event",
	"client-ping-event",
	"channel-update-event",
}

var EventNameMap map[string]EventId

func LookupEventName(name string) EventId {
	if EventNameMap == nil {
		EventNameMap = make(map[string]EventId)
		for i, n := range EventNames {
			EventNameMap[n] = EventId(i)
		}
	}
	return EventNameMap[name]
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

// Event
//
//
type Event struct {
	Id    EventId
	Value ValuePacker
}

type ValueUnpackerFunction func(EventId, []byte) (ValuePacker, error)

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

//
// MISC
//

func PackLength(size uint) []byte {
	dest := make([]byte, 5)

	if size <= 0x80 {
		dest[0] = byte(size)
		return dest[:1]
	}
	if size < 0x100 {
		dest[0] = 0x81
		dest[1] = byte(size)
		return dest[:2]
	}
	if size < 0x10000 {
		dest[0] = 0x82
		dest[1] = byte(size >> 8)
		dest[2] = byte(size)
		return dest[:3]
	}
	if size < 0x1000000 {
		dest[0] = 0x83
		dest[1] = byte(size >> 16)
		dest[2] = byte(size >> 8)
		dest[3] = byte(size)
		return dest[:4]
	}
	dest[0] = 0x84
	dest[1] = byte(size >> 24)
	dest[2] = byte(size >> 16)
	dest[3] = byte(size >> 8)
	dest[4] = byte(size)
	return dest
}

func PackEvent(id EventId, data []byte) []byte {
	dest := make([]byte, 0, 6+len(data))

	dest = append(dest, byte(id))
	dest = append(dest, PackLength(uint(len(data)))...)
	dest = append(dest, data...)
	return dest
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
