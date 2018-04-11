package socket

import (
	"encoding/hex"
	"fmt"
	"io"
)

type EventId byte

type ValuePacker interface {
	PackValue() ([]byte, error)
}

// Event
//
//
type Event struct {
	Id    EventId
	Value interface{}
}

func NewEvent(eid EventId, value interface{}) *Event {
	return &Event{Id: eid, Value: value}
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

func dpacket(d []byte) string {
	h := make([]byte, 0, 20)

	if len(d) <= 20 {
		h = h[:len(d)]
		copy(h, d)

		return fmt.Sprintf("[%d] %s", len(d), hex.EncodeToString(h))
	}
	h = h[:16]
	copy(h, d)
	h = h[:20]
	copy(h[16:], d[len(d)-4:])

	return fmt.Sprintf("[%d] %s ... %s", len(d), hex.EncodeToString(h[:16]), hex.EncodeToString(h[16:]))
}

func EncodeToStream(w io.Writer, eid EventId, value interface{}) error {
	var pv []byte
	var err error

	if value != nil {
		switch v := value.(type) {
		case ValuePacker:
			pv, err = v.PackValue()
			if err != nil {
				return err
			}
		case []byte:
			pv = v
		case byte:
			pv = []byte{v}
		case bool:
			if v {
				pv = []byte{1}
			} else {
				pv = []byte{0}
			}
		default:
			return fmt.Errorf("Do not know how to encode a %T, for event %d", value, eid)
		}
	} else {
		pv = []byte{}
	}

	dest := make([]byte, 0, 6+len(pv)) // prealloc
	dest = append(dest, byte(eid))
	dest = append(dest, PackLength(uint(len(pv)))...)
	dest = append(dest, pv...)

	//fmt.Printf("# Sending %s\n", dpacket(dest))
	_, err = w.Write(dest)
	if err != nil {
		return fmt.Errorf("Failed to write %d bytes for value of encoded event %d, %s", len(dest), eid, err)
	}
	return nil
}

func DecodeFromStream(r io.Reader) (EventId, []byte, error) {
	var rbuf [4]byte
	var rlen uint
	var err error

	_, err = io.ReadFull(r, rbuf[:2])
	if err != nil {
		return 0, nil, err
	}

	eventId := EventId(rbuf[0])

	if rbuf[1] <= 0x80 {
		rlen = uint(rbuf[1])
	} else {
		switch rbuf[1] & 0xF {
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
			return 0, nil, fmt.Errorf("Wrong byte length in event decoding (got %d)", rbuf[1])
		}
		if err != nil {
			return 0, nil, fmt.Errorf("Length read failed in event decoding, %s", err)
		}
	}

	dbuf := make([]byte, rlen)
	re, err := io.ReadFull(r, dbuf)
	if err != nil {
		return 0, nil, fmt.Errorf("Expected %d bytes, but got %d bytes while decoding value for event %d (%s), %s", rlen, re, eventId, eventId, err)
	}
	//fmt.Printf("# Receiving %d bytes for eid=%d, %s\n", rlen, eventId, dpacket(dbuf))

	return eventId, dbuf, nil
}
