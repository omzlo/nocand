package socket

import (
	"fmt"
	"github.com/omzlo/clog"
	"io"
)

type EventId byte

type Eventer interface {
	Id() EventId
	MsgId() uint16
	SetMsgId(uint16)
	Pack() ([]byte, error)
	Unpack([]byte) error
	String() string
}

// Event
//
//
type BaseEvent struct {
	msgId   uint16
	eventId EventId
}

func (e BaseEvent) Id() EventId {
	return e.eventId
}

func (e BaseEvent) MsgId() uint16 {
	return e.msgId
}

func (e *BaseEvent) SetMsgId(mid uint16) {
	e.msgId = mid
}

func (e BaseEvent) String() string {
	return fmt.Sprintf("%s(%d)\t", e.eventId, e.eventId)
}

//
// MISC
//

func packLength(size uint) []byte {
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

func EncodeEvent(w io.Writer, e Eventer) error {
	var pv []byte
	var err error

	pv, err = e.Pack()
	if err != nil {
		return err
	}

	dest := make([]byte, 0, 8+len(pv)) // prealloc
	dest = append(dest, byte(e.MsgId()>>8))
	dest = append(dest, byte(e.MsgId()))
	dest = append(dest, byte(e.Id()))
	dest = append(dest, packLength(uint(len(pv)))...)
	dest = append(dest, pv...)

	fmt.Printf(">>> Sending %q\n", dest)

	_, err = w.Write(dest)
	if err != nil {
		return fmt.Errorf("Failed to write %d bytes for value of encoded event %d, %s", len(dest), e.Id(), err)
	}
	return nil
}

func DecodeEvent(r io.Reader) (Eventer, error) {
	var rbuf [4]byte
	var err error
	var rlen uint

	_, err = io.ReadFull(r, rbuf[:])
	if err != nil {
		return nil, err
	}

	fmt.Printf(">>> Receiving %q ", rbuf[:])

	msgId := (uint16(rbuf[0]) << 8) | uint16(rbuf[1])
	eventId := EventId(rbuf[2])

	if rbuf[3] <= 0x80 {
		rlen = uint(rbuf[3])
	} else {
		switch rbuf[3] & 0xF {
		case 1:
			_, err = io.ReadFull(r, rbuf[:1])
			rlen = uint(rbuf[0])
		case 2:
			_, err = io.ReadFull(r, rbuf[:2])
			rlen = (uint(rbuf[0]) << 8) | uint(rbuf[1])
		case 3:
			_, err = io.ReadFull(r, rbuf[:3])
			rlen = (uint(rbuf[0]) << 16) | (uint(rbuf[1]) << 8) | uint(rbuf[2])
		case 4:
			_, err = io.ReadFull(r, rbuf[:4])
			rlen = (uint(rbuf[0]) << 24) | (uint(rbuf[1]) << 16) | (uint(rbuf[2]) << 8) | uint(rbuf[3])
		default:
			return nil, fmt.Errorf("Wrong byte length in event decoding (got %d)", rbuf[1])
		}
		if err != nil {
			return nil, fmt.Errorf("Length read failed in event decoding, %s", err)
		}
	}

	dbuf := make([]byte, rlen)
	re, err := io.ReadFull(r, dbuf)
	if err != nil {
		return nil, fmt.Errorf("Expected %d bytes, but got %d bytes while decoding value for event %d (%s), %s", rlen, re, eventId, eventId, err)
	}
	fmt.Printf("%q\n", dbuf[:rlen])

	clog.DebugXX("Got message %d with event %s(%d) and %d bytes of payload", msgId, eventId, eventId, rlen)

	var x Eventer
	switch eventId {
	case ClientHelloEventId:
		x = NewClientHelloEvent("", 0, 0)
	case ServerHelloEventId:
		x = NewServerHelloEvent("", 9, 0)
	case ChannelFilterEventId:
		x = NewChannelFilterEvent()
	case ServerAckEventId:
		x = NewServerAckEvent(0)
	case BusPowerStatusUpdateEventId:
		x = NewBusPowerStatusUpdateEvent(nil)
	case BusPowerEventId:
		x = NewBusPowerEvent(true)
	case ChannelUpdateRequestEventId:
		x = NewChannelUpdateRequestEvent("", 0)
	case ChannelUpdateEventId:
		x = NewChannelUpdateEvent("", 0, 0, nil)
	case ChannelListRequestEventId:
		x = NewChannelListRequestEvent()
	case ChannelListEventId:
		x = NewChannelListEvent()
	case NodeUpdateRequestEventId:
		x = NewNodeUpdateRequestEvent(0)
	case NodeUpdateEventId:
		x = new(NodeUpdateEvent)
	case NodeListRequestEventId:
		x = NewNodeListRequestEvent()
	case NodeListEventId:
		x = NewNodeListEvent()
	case NodeFirmwareDownloadRequestEventId:
		x = NewNodeFirmwareDownloadRequestEvent(0)
	case NodeFirmwareEventId:
		x = NewNodeFirmwareEvent(0)
	case NodeFirmwareProgressEventId:
		x = NewNodeFirmwareProgressEvent(0)
	case NodeRebootRequestEventId:
		x = NewNodeRebootRequestEvent(0, false)
	case BusPowerStatusUpdateRequestEventId:
		x = NewBusPowerStatusUpdateRequestEvent()
	case DeviceInformationRequestEventId:
		x = NewDeviceInformationRequestEvent()
	case DeviceInformationEventId:
		x = NewDeviceInformationEvent(nil)
	case SystemPropertiesRequestEventId:
		x = NewSystemPropertiesRequestEvent()
	case SystemPropertiesEventId:
		x = NewSystemPropertiesEvent(nil)
	default:
		return nil, fmt.Errorf("Unprocessable event %d with %d bytes of payload", eventId, rlen)
	}

	x.SetMsgId(msgId)

	if err = x.Unpack(dbuf); err != nil {
		return nil, err
	}

	return x, nil
}
