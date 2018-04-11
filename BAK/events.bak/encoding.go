package events

import (
//"errors"
//"math"
)

/*
func EncodeUint32(b []byte, uint32 u) error {
	if len(b) < 4 {
		return errors.New("Encoding buffer must be 4 bytes at least")
	}
	b[0] = byte(u >> 24)
	b[1] = byte(u >> 16)
	b[2] = byte(u >> 8)
	b[3] = byte(u)
	return nil
}

func EncodeFloat32(b []byte, float32 f) error {
	u := math.Float32bits(f)
	return EncodeUint32(b, u)
}

func EncodeUint16(b []byte, uint32 u) error {
	if len(b) < 2 {
		return errors.New("Encoding buffer must be 2 bytes at least")
	}
	b[0] = byte(u >> 8)
	b[1] = byte(u)
	return nil
}

func DecodeUint32(b []byte) (uint32, error) {
	if len(b) < 4 {
		return 0, errors.New("Decoding buffer must be 4 bytes at least")
	}
	return (uint32(b[0]) << 24) | (uint32(b[1]) << 16) | (uint32(b[2]) << 8) | uint32(b[3]), nil
}

func DecodeFloat32(b []byte) (float32, error) {
	u, err := DecodeUint32(b)
	if err != nil {
		return err
	}
	return math.Float32frombits(u), nil
}

func DecodeUint16(b []byte) (uint16, error) {
	if len(b) < 2 {
		return 0, errors.New("Decoding buffer must be 2 bytes at least")
	}
	return (uint16(b[0]) << 8) | uint16(b[1]), nil
}
*/

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

/*
func DecodeLength(src []byte) (size uint, size_of_size uint) {
	if src[0] <= 0x80 {
		return uint(src[0]), 1
	}
	if src[0] == 0x81 {
		return uint(src[1]), 2
	}
	if src[0] == 0x82 {
		return (uint(src[2]) << 8) | uint(src[1]), 3
	}
	if src[0] == 0x83 {
		return (uint(src[3]) << 16) | (uint(src[2]) << 8) | uint(src[1]), 4
	}
	if src[0] == 0x84 {
		return (uint(src[4]) << 24) | (uint(src[3]) << 16) | (uint(src[2]) << 8) | uint(src[1]), 5
	}
	return 0, 0
}
*/
