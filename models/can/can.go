package can

import (
	"fmt"
)

type Frame struct {
	CanId uint32
	Dlc   uint8
	Data  [8]uint8
}

const (
	CANID_MASK_EXTENDED = (1 << 31)
	CANID_MASK_REMOTE   = (1 << 30)
	CANID_MASK_ERROR    = (1 << 29)
	CANID_MASK_CONTROL  = (CANID_MASK_EXTENDED | CANID_MASK_REMOTE | CANID_MASK_ERROR)
)

func (frame *Frame) PackValue() ([]byte, error) {
	buf := make([]byte, 13)
	if err := EncodeFrame(frame, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func EncodeFrame(frame *Frame, buf []byte) error {
	if len(buf) < 13 {
		return fmt.Errorf("Encode buffer must be at least 13 bytes, found only %d", len(buf))
	}
	buf[0] = byte(frame.CanId >> 24)
	buf[1] = byte(frame.CanId >> 16)
	buf[2] = byte(frame.CanId >> 8)
	buf[3] = byte(frame.CanId)
	buf[4] = frame.Dlc
	buf[5] = frame.Data[0]
	buf[6] = frame.Data[1]
	buf[7] = frame.Data[2]
	buf[8] = frame.Data[3]
	buf[9] = frame.Data[4]
	buf[10] = frame.Data[5]
	buf[11] = frame.Data[6]
	buf[12] = frame.Data[7]
	return nil
}

func DecodeFrame(buf []byte) (*Frame, error) {
	frame := new(Frame)

	if len(buf) < 13 {
		return nil, fmt.Errorf("Slice length too short for decoding operation, required 13 bytes, got %d", len(buf))
	}
	frame.CanId = uint32(buf[0]) << 24
	frame.CanId |= uint32(buf[1]) << 16
	frame.CanId |= uint32(buf[2]) << 8
	frame.CanId |= uint32(buf[3])
	frame.Dlc = buf[4]
	frame.Data[0] = buf[5]
	frame.Data[1] = buf[6]
	frame.Data[2] = buf[7]
	frame.Data[3] = buf[8]
	frame.Data[4] = buf[9]
	frame.Data[5] = buf[10]
	frame.Data[6] = buf[11]
	frame.Data[7] = buf[12]
	return frame, nil
}

func (frame *Frame) String() string {
	dlc := frame.Dlc
	s := "<"

	if frame.CanId&CANID_MASK_ERROR != 0 {
		s = "ERR"
	} else {
		if frame.CanId&CANID_MASK_EXTENDED != 0 {
			s = "EXT"
		} else {
			s = "STD"
		}
		if frame.CanId&CANID_MASK_REMOTE != 0 {
			s += "+REM"
		}
	}

	s += fmt.Sprintf("@%x ", frame.CanId)
	if dlc > 8 {
		s += fmt.Sprintf("!%d:", dlc)
		dlc = 8
	} else {
		s += fmt.Sprintf("%d:", dlc)
	}

	for i := uint8(0); i < dlc; i++ {
		s += fmt.Sprintf(" %02x", frame.Data[i])
	}
	return s + ">"
}

func (frame *Frame) IsExtended() bool {
	return (frame.CanId & CANID_MASK_EXTENDED) != 0
}

func (frame *Frame) IsError() bool {
	return (frame.CanId >> CANID_MASK_ERROR) != 0
}

func (frame *Frame) IsRemote() bool {
	return (frame.CanId >> CANID_MASK_REMOTE) != 0
}

/*
const QUEUE_SIZE = 128

type FrameQueue struct {
	Front uint
	Back  uint
	Queue [QUEUE_SIZE]*Frame
}
func NewFrameQueue() *FrameQueue {
	return &FrameQueue{Font: 0, Back: 0}
}

func (q *FrameQueue) IsEmpty() bool {
	return q.Front == a.Back
}

func (q *FrameQueue) IsFull() bool {
	return (q.Back+1)&(QUEUE_SIZE-1) == q.Front
}

func (q *FrameQueue) PushBack(f *Frame) bool {
	if q.IsFull() {
		return false
	}
	q.Queue[q.Back] = f
	q.Back = (q.Back + 1) & (QUEUE_SIZE - 1)
	return true
}

func (q *FrameQueue) PopFront() *Frame {
	if q.IsEmpty() {
		return nil
	}
	res := q.Queue[q.Front]
	q.Front = (q.Front + 1) & (QUEUE_SIZE - 1)
	return res
}
*/
