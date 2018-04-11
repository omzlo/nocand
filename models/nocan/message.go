package nocan

import (
	"encoding/hex"
	"fmt"
	"time"
)

const (
	MAX_NODE_N           = 128
	NOCANID_MASK_FIRST   = (1 << 28)
	NOCANID_MASK_LAST    = (1 << 20)
	NOCANID_MASK_SYSTEM  = (1 << 18)
	NOCANID_MASK_MESSAGE = ^(uint32((1 << 28) | (1 << 20)))
)

const (
	DEFAULT_TIMEOUT  = 3 * time.Second
	EXTENDED_TIMEOUT = 12 * time.Second
)

const (
	NO_NODE    NodeId    = NodeId(-1)
	NO_CHANNEL ChannelId = ChannelId(0xFFFF)
)

type ChannelId uint16

func (cid ChannelId) ToBytes() []byte {
	return []byte{byte(cid >> 8), byte(cid)}
}

type NodeId int8

type Message struct {
	CanId uint32
	Dlc   uint8
	Data  [64]byte
}

/*
func (m *Message) Id() events.EventId {
	if m.IsSystemMessage() {
		fn, _ := m.SystemFunctionParam()
		return events.EventId(fn) + events.NocanPublishEvent
	}
	return events.NocanPublishEvent
}
*/
/*
func (m *Message) PackValue() ([]byte, error) {
	buf := make([]byte, 5+m.Dlc)

	buf[0] = byte(m.CanId >> 24)
	buf[1] = byte(m.CanId >> 16)
	buf[2] = byte(m.CanId >> 8)
	buf[3] = byte(m.CanId)
	buf[4] = m.Dlc
	copy(buf[5:], m.Data[:m.Dlc])

	return buf, nil
}

func UnpackMessage(id events.EventId, value []byte) (events.ValuePacker, error) {
	m := new(Message)
	if len(value) < 5 {
		return nil, fmt.Errorf("Encoded message too short, must be at least 5 bytes, got %d", len(value))
	}
	m.CanId = (uint32(value[0]) << 24) | (uint32(value[1]) << 16) | (uint32(value[2]) << 8) | uint32(value[3])
	m.Dlc = uint8(value[4])
	if m.Dlc > 64 {
		return nil, fmt.Errorf("Encoded message body cannot exceed 64 bytes, got %d", m.Dlc)
	}
	if len(value) < 5+int(m.Dlc) {
		return nil, fmt.Errorf("Encoded message too short, DLC is %d, but only %d bytes available", m.Dlc, len(value)-5)
	}
	copy(m.Data[:], value[5:5+m.Dlc])
	return m, nil
}
*/

func NewMessage(id uint32, data []byte) *Message {
	m := new(Message)
	m.CanId = id
	if data != nil {
		if len(data) > 64 {
			data = data[:64]
		}
		m.Dlc = uint8(len(data))
		copy(m.Data[:], data)
	} else {
		m.Dlc = 0
	}
	return m
}

func NewSystemMessage(node NodeId, fn MessageType, param uint8, data []byte) *Message {
	return NewMessage(NOCANID_MASK_SYSTEM|(uint32(node)<<21)|(uint32(fn)<<8)|uint32(param), data)
}

func NewPublishMessage(node NodeId, channel ChannelId, data []byte) *Message {
	return NewMessage((uint32(node)<<21)|uint32(channel), data)
}

func (m *Message) IsSystemMessage() bool {
	return (m.CanId & NOCANID_MASK_SYSTEM) != 0
}

func (m *Message) NodeId() NodeId {
	return NodeId((m.CanId >> 21) & 0x7F)
}

func (m *Message) SetNodeId(nodeid NodeId) *Message {
	m.CanId &= ^uint32(0x7F << 21)
	m.CanId |= uint32(nodeid) << 21
	return m
}

func (m *Message) ChannelId() ChannelId {
	return ChannelId(m.CanId & 0xFFFF)
}

func (m *Message) SetChannelId(cid ChannelId) *Message {
	m.CanId &= ^uint32(0xFFFF)
	m.CanId |= uint32(cid)
	return m
}

func (m *Message) SystemFunctionParam() (MessageType, uint8) {
	return MessageType((m.CanId >> 8) & 0xFF), uint8(m.CanId & 0xFF)
}

func (m *Message) SetSystemFunctionParam(fn MessageType, pr uint8) *Message {
	m.CanId &= ^uint32(0xFFFF)
	m.CanId |= uint32(fn)<<8 | uint32(pr)
	return m
}

func (m *Message) AppendData(data []byte) bool {
	if int(m.Dlc)+len(data) > 64 {
		return false
	}
	copy(m.Data[m.Dlc:], data)
	m.Dlc += uint8(len(data))
	return true
}

func (m *Message) String() string {
	if m.IsSystemMessage() {
		fn, pn := m.SystemFunctionParam()
		return fmt.Sprintf("<%s node=%d, func=%d, param=%d, len=%d, data=%s>", MessageType(fn), m.NodeId(), fn, pn, m.Dlc, hex.EncodeToString(m.Bytes()))
	}
	return fmt.Sprintf("<PUBLISH node=%d, channel=%d, len=%d, data=%s>", m.NodeId(), m.ChannelId(), m.Dlc, hex.EncodeToString(m.Bytes()))
}

func (m *Message) Bytes() []byte {
	return m.Data[:m.Dlc]
}

func (m *Message) DataToString() string {
	return string(m.Bytes())
}

/*********


func SendMessage(m *Message) error {
	var frame can.Frame
	var pos uint8

	clog.DebugX("** Sending %s **", m)
	pos = 0
	for {
		frame.CanId = m.CanId | can.CANID_MASK_EXTENDED
		if pos == 0 {
			frame.CanId |= NOCANID_MASK_FIRST
		}
		if m.Dlc-pos > 8 {
			frame.Dlc = 8
		} else {
			frame.Dlc = m.Dlc - pos
			frame.CanId |= NOCANID_MASK_LAST
		}
		copy(frame.Data[:], m.Data[pos:pos+frame.Dlc])
		if err := rpi.DriverSendCanFrame(&frame); err != nil {
			return err
		}
		pos += frame.Dlc
		if pos >= m.Dlc {
			break
		}
	}
	return nil
}

func SendSystemMessage(node NodeId, fn MessageType, pm uint8, data []byte) error {
	msg := NewSystemMessage(node, fn, pm, data)
	return SendMessage(msg)
}

func SendPublishMessage(node NodeId, channel ChannelId, data []byte) error {
	msg := NewPublishMessage(node, channel, data)
	return SendMessage(msg)
}
*/
/*
func WaitForSystemMessage(node NodeId, fn uint8, timeout time.Duration) *Message {
	result := make(chan *Message)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	callback := func(m *Message) {
		if m.IsSystemMessage() {
			mfn, _ := m.SystemFunctionParam()
			if mfn == fn {
				// should we also set a flag to avoid re-entry ?
				result <- m
			}
		}
	}

	core.DefaultEventManager.AddListener("nocan-recv", callback)
	defer core.DefaultEventManager.RemoveListener("nocan-recv", callback)

	select {
	case m := <-result:
		return m
	case <-ticker.C: // timeout
		return nil
	}
}

var messagePending [MAX_NODE_N]*Message

func AssembleCanMessage(frame *can.Frame) {
	node := (frame.CanId >> 21) & 0x7F

	if !frame.IsExtended() {
		clog.Warning("Expected extended CAN frame, discarding %s.", frame)
		return
	}

	if (frame.CanId & NOCANID_MASK_FIRST) != 0 {
		if messagePending[node] != nil {
			clog.Warning("Got frame with inconsistent first bit indicator, discarding.")
			return
		}
		messagePending[node] = NewMessage(frame.CanId, frame.Data[:frame.Dlc])
	} else {
		if messagePending[node] == nil {
			clog.Warning("Got first frame with missing first bit indicator, discarding.")
			return
		}
		messagePending[node].AppendData(frame.Data[:frame.Dlc])
	}
	if (frame.CanId & NOCANID_MASK_LAST) != 0 {
		msg := messagePending[node]
		clog.Info("** Received %s **", msg)
		core.DefaultEventManager.Emit("nocan-recv", msg)
		if msg.IsSystemMessage() {
			fn, _ := msg.SystemFunctionParam()
			core.DefaultEventManager.Emit(SystemFunctionString(fn), msg)
		} else {
			core.DefaultEventManager.Emit("nocan-publish", msg)
		}
		messagePending[node] = nil // clear
	}
}

func DebugMessage(m *Message) {
	_, seq := m.SystemFunctionParam()
	clog.Warning("DEBUG[%d]: %s", seq, string(m.Data[:m.Dlc]))
}

func init() {
	core.DefaultEventManager.On("can-recv", AssembleCanMessage)
	core.DefaultEventManager.On("nocan-sys-debug-message", DebugMessage)
}
*/
