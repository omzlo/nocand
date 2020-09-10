package socket

import (
	//"encoding/hex"
	"fmt"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/properties"
	"io"
)

type EventId byte

type Packer interface {
	Pack() ([]byte, error)
	Unpack([]byte) error
}

// Event
//
//
type Event struct {
	MsgId uint16
	Id    EventId
	Value Packer
}

func NewEvent(msg_id uint16, eid EventId, value Packer) *Event {
	return &Event{MsgId: msg_id, Id: eid, Value: value}
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

func (e *Event) WriteTo(w io.Writer) (int64, error) {
	var pv []byte
	var err error

	pv, err = e.Value.Pack()
	if err != nil {
		return 0, err
	}

	dest := make([]byte, 0, 8+len(pv)) // prealloc
	dest = append(dest, byte(e.MsgId>>8))
	dest = append(dest, byte(e.MsgId))
	dest = append(dest, byte(e.Id))
	dest = append(dest, packLength(uint(len(pv)))...)
	dest = append(dest, pv...)

	_, err = w.Write(dest)
	if err != nil {
		return 0, fmt.Errorf("Failed to write %d bytes for value of encoded event %d, %s", len(dest), e.Id, err)
	}
	return int64(len(dest)), nil
}

func (e *Event) ReadFrom(r io.Reader) (int64, error) {
	var rbuf [4]byte
	var err error
	var rlen uint
	var len_of_len uint

	_, err = io.ReadFull(r, rbuf[:])
	if err != nil {
		return 0, err
	}

	msgId := uint16(rbuf[1]<<8) | uint16(rbuf[0])
	eventId := EventId(rbuf[2])

	if rbuf[3] <= 0x80 {
		rlen = uint(rbuf[3])
		len_of_len = 1
	} else {
		switch rbuf[3] & 0xF {
		case 1:
			_, err = io.ReadFull(r, rbuf[:1])
			rlen = uint(rbuf[0])
			len_of_len = 2
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
			return 0, fmt.Errorf("Wrong byte length in event decoding (got %d)", rbuf[1])
		}
		if err != nil {
			return 0, fmt.Errorf("Length read failed in event decoding, %s", err)
		}
	}

	dbuf := make([]byte, rlen)
	re, err := io.ReadFull(r, dbuf)
	if err != nil {
		return 0, fmt.Errorf("Expected %d bytes, but got %d bytes while decoding value for event %d (%s), %s", rlen, re, eventId, eventId, err)
	}
	e.MsgId = msgId
	e.Id = eventId

	var x Packer
	switch e.Id {
	case ClientHelloEvent:
		x = new(ClientHello)
		break
	case ServerHelloEvent:
		x = new(ServerHello)
		break
	case ChannelSubscribeEvent:
		x = new(ChannelSubscriptionList)
		break
	case ServerAckEvent:
		x = new(ServerAck)
		break
	case BusPowerStatusUpdateEvent:
		x = new(device.PowerStatus)
		break
	case BusPowerEvent:
		x = new(BusPower)
		break
	case ChannelUpdateRequestEvent:
		x = new(ChannelUpdateRequest)
		break
	case ChannelUpdateEvent:
		x = new(ChannelUpdate)
		break
	case ChannelListRequestEvent:
		x = new(ChannelListRequest)
		break
	case ChannelListEvent:
		x = new(ChannelList)
		break
	case NodeUpdateRequestEvent:
		x = new(NodeUpdateRequest)
		break
	case NodeUpdateEvent:
		x = new(NodeUpdate)
		break
	case NodeListRequestEvent:
		x = new(NodeListRequest)
		break
	case NodeListEvent:
		x = new(NodeList)
		break
	case NodeFirmwareUploadEvent:
		x = new(NodeFirmware)
		break
	case NodeFirmwareDownloadRequestEvent:
		x = new(NodeFirmwareDownloadRequest)
		break
	case NodeFirmwareDownloadEvent:
		x = new(NodeFirmware)
		break
	case NodeFirmwareProgressEvent:
		x = new(NodeFirmwareProgress)
		break
	case NodeRebootRequestEvent:
		x = new(NodeRebootRequest)
		break
	case BusPowerStatusUpdateRequestEvent:
		x = new(BusPowerStatusUpdateRequest)
		break
	case DeviceInformationRequestEvent:
		x = new(DeviceInformationRequest)
		break
	case DeviceInformationEvent:
		x = new(device.Information)
		break
	case SystemPropertiesRequestEvent:
		x = new(SystemPropertiesRequest)
		break
	case SystemPropertiesEvent:
		x = properties.New()
		break
	default:
		return 0, fmt.Errorf("Unprocessable event %d with %d bytes of payload", e.Id, rlen)
	}

	if err = x.Unpack(dbuf); err != nil {
		return 0, err
	}
	e.Value = x
	return int64(3 + len_of_len + rlen), nil
}
