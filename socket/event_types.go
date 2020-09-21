package socket

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/properties"
	"strconv"
	"time"
)

var ErrorMissingData error = errors.New("Missing data for value decoder")

func EncodeUint16(dest []byte, u uint16) {
	dest[0] = byte(u >> 8)
	dest[1] = byte(u)
}

func DecodeUint16(src []byte) uint16 {
	return (uint16(src[0]) << 8) | uint16(src[1])
}

func EncodeUint32(dest []byte, u uint32) {
	dest[0] = byte(u >> 24)
	dest[1] = byte(u >> 16)
	dest[2] = byte(u >> 8)
	dest[3] = byte(u)
}

func DecodeUint32(src []byte) uint32 {
	return (uint32(src[0]) << 24) |
		(uint32(src[1]) << 16) |
		(uint32(src[2]) << 8) |
		uint32(src[3])
}

func EncodeUint64(dest []byte, u uint64) {
	dest[0] = byte(u >> 56)
	dest[1] = byte(u >> 48)
	dest[2] = byte(u >> 40)
	dest[3] = byte(u >> 32)
	dest[4] = byte(u >> 24)
	dest[5] = byte(u >> 16)
	dest[6] = byte(u >> 8)
	dest[7] = byte(u)
}

func DecodeUint64(src []byte) uint64 {
	return (uint64(src[0]) << 56) |
		(uint64(src[1]) << 48) |
		(uint64(src[2]) << 40) |
		(uint64(src[3]) << 32) |
		(uint64(src[4]) << 24) |
		(uint64(src[5]) << 16) |
		(uint64(src[6]) << 8) |
		uint64(src[7])
}

/****************************************************************************/
// EmptyRequest

type EmptyEvent struct {
	BaseEvent
}

func (x EmptyEvent) Pack() ([]byte, error) {
	return make([]byte, 0), nil
}

func (c *EmptyEvent) Unpack(b []byte) error {
	return nil
}

/****************************************************************************/

type ClientHelloEvent struct {
	BaseEvent
	Tool         string
	VersionMajor byte
	VersionMinor byte
}

func NewClientHelloEvent(tool string, major byte, minor byte) *ClientHelloEvent {
	return &ClientHelloEvent{BaseEvent: BaseEvent{0, ClientHelloEventId}, Tool: tool, VersionMajor: major, VersionMinor: minor}
}

func (ch *ClientHelloEvent) Pack() ([]byte, error) {
	b := make([]byte, 0, len(ch.Tool)+3)
	b = append(b, byte(len(ch.Tool)))
	b = append(b, []byte(ch.Tool)...)
	b = append(b, ch.VersionMajor)
	b = append(b, ch.VersionMinor)
	return b, nil
}

func (ch *ClientHelloEvent) Unpack(b []byte) error {
	if int(b[0]+2) > len(b) {
		return ErrorMissingData
	}
	ch.Tool = string(b[1 : 1+b[0]])
	ch.VersionMajor = b[b[0]]
	ch.VersionMinor = b[b[0]+1]
	return nil
}

/****************************************************************************/

type ServerHelloEvent struct {
	ClientHelloEvent
}

func NewServerHelloEvent(tool string, major byte, minor byte) *ServerHelloEvent {
	return &ServerHelloEvent{ClientHelloEvent{BaseEvent: BaseEvent{0, ServerHelloEventId}, Tool: tool, VersionMajor: major, VersionMinor: minor}}
}

/****************************************************************************/
// ChannelFilter restricts the channels that a client will receive.
// By default, the client receives all channels. By adding a filter, this can be restricted.
//

type ChannelFilterEvent struct {
	BaseEvent
	Channels map[nocan.ChannelId]bool
}

func NewChannelFilterEvent(chans ...nocan.ChannelId) *ChannelFilterEvent {
	clist := &ChannelFilterEvent{BaseEvent: BaseEvent{0, ChannelFilterEventId}, Channels: make(map[nocan.ChannelId]bool)}
	clist.Add(chans...)
	return clist
}

func (sl *ChannelFilterEvent) Add(chans ...nocan.ChannelId) {
	for _, e := range chans {
		sl.Channels[e] = true
	}
}

func (sl *ChannelFilterEvent) Remove(chans ...nocan.ChannelId) {
	for _, e := range chans {
		delete(sl.Channels, e)
	}
}

func (sl *ChannelFilterEvent) Includes(id nocan.ChannelId) bool {
	return sl.Channels[id]
}

func (sl *ChannelFilterEvent) Pack() ([]byte, error) {
	p := make([]byte, len(sl.Channels)*2)

	i := 0
	for k, _ := range sl.Channels {
		EncodeUint16(p[i:], uint16(k))
		i += 2
	}
	return p, nil
}

func (sl *ChannelFilterEvent) Unpack(b []byte) error {
	for i := 0; i < len(b); i += 2 {
		x := nocan.ChannelId(DecodeUint16(b[i:]))
		sl.Add(x)
	}
	return nil
}

func (sl *ChannelFilterEvent) String() string {
	s := sl.BaseEvent.String() + "["

	var i int = 0
	for k, _ := range sl.Channels {
		if k == nocan.UNDEFINED_CHANNEL {
			s += "*"
		} else {
			s += strconv.Itoa(int(k))
		}
		if i < len(sl.Channels)-1 {
			s += ", "
		}
		i++
	}
	return s + "]"
}

/****************************************************************************/
// ServerAck
//
//

type ServerAckEvent struct {
	BaseEvent
	Code byte
}

const (
	ServerAckSuccess        = 0
	ServerAckBadRequest     = 1
	ServerAckUnauthorized   = 2
	ServerAckNotFound       = 3
	ServerAckGeneralFailure = 4
	ServerAckTimeout        = 5
)

var serverAckStrings = [6]string{
	"Success",
	"Bad request",
	"Unauthorized",
	"Not found",
	"General failure",
	"Timeout",
}

var (
	ErrorServerAckBadRequest     = errors.New("Bad request")
	ErrorServerAckUnauthorized   = errors.New("Unauthorized")
	ErrorServerAckNotFound       = errors.New("Not found")
	ErrorServerAckGeneralFailure = errors.New("General failure")
	ErrorServerAckTimeout        = errors.New("Timeout")
	ErrorServerAckUndefined      = errors.New("Undefined")
)

func NewServerAckEvent(val byte) *ServerAckEvent {
	return &ServerAckEvent{BaseEvent: BaseEvent{0, ServerAckEventId}, Code: val}
}

func (sa *ServerAckEvent) Pack() ([]byte, error) {
	p := make([]byte, 1)
	p[0] = sa.Code
	return p, nil
}

func (sa *ServerAckEvent) Unpack(value []byte) error {
	if len(value) < 1 {
		return ErrorMissingData
	}
	sa.Code = value[0]
	return nil
}

func (sa ServerAckEvent) String() string {
	if sa.Code < 6 {
		return serverAckStrings[sa.Code]
	}
	return sa.BaseEvent.String() + "Unknown error"
}

func (sa ServerAckEvent) ToError() error {
	switch sa.Code {
	case 0:
		return nil
	case 1:
		return ErrorServerAckBadRequest
	case 2:
		return ErrorServerAckUnauthorized
	case 3:
		return ErrorServerAckNotFound
	case 4:
		return ErrorServerAckGeneralFailure
	case 5:
		return ErrorServerAckTimeout
	}
	return ErrorServerAckUndefined
}

/****************************************************************************/

// ChannelUpdateRequest
//
//

type ChannelUpdateRequestEvent struct {
	BaseEvent
	ChannelId   nocan.ChannelId
	ChannelName string
}

func NewChannelUpdateRequestEvent(chan_name string, chan_id nocan.ChannelId) *ChannelUpdateRequestEvent {
	return &ChannelUpdateRequestEvent{BaseEvent: BaseEvent{0, ChannelUpdateRequestEventId}, ChannelId: chan_id, ChannelName: chan_name}
}

func (cu *ChannelUpdateRequestEvent) PackedLength() int {
	return len(cu.ChannelName) + 3
}

func (cu *ChannelUpdateRequestEvent) Pack() ([]byte, error) {
	buf := make([]byte, 0, cu.PackedLength())
	buf = append(buf, byte(cu.ChannelId>>8), byte(cu.ChannelId&0xFF))
	buf = append(buf, byte(len(cu.ChannelName)))
	buf = append(buf, []byte(cu.ChannelName)...)
	return buf, nil
}

func (cu *ChannelUpdateRequestEvent) Unpack(value []byte) error {
	if len(value) < 3 {
		return ErrorMissingData
	}
	cu.ChannelId = (nocan.ChannelId(value[0]) << 8) | nocan.ChannelId(value[1])

	lName := int(value[2])
	if lName > 64 {
		return errors.New("Channel name exceeds 64 bytes")
	}
	value = value[3:]
	if lName > len(value) {
		return ErrorMissingData
	}
	cu.ChannelName = string(value[:lName])
	return nil
}

func (cu ChannelUpdateRequestEvent) String() string {
	if cu.ChannelName == "" {
		return fmt.Sprintf("%s#%d", cu.BaseEvent.String(), cu.ChannelId)
	}
	return cu.BaseEvent.String() + cu.ChannelName
}

/****************************************************************************/

// ChannelUpdate
//
//
type ChannelStatus byte

const (
	CHANNEL_CREATED ChannelStatus = iota
	CHANNEL_UPDATED
	CHANNEL_DESTROYED
	CHANNEL_NOT_FOUND
)

func (cs ChannelStatus) String() string {
	switch cs {
	case CHANNEL_CREATED:
		return "created"
	case CHANNEL_UPDATED:
		return "updated"
	case CHANNEL_DESTROYED:
		return "destroyed"
	case CHANNEL_NOT_FOUND:
		return "not found"
	default:
		return "!unknown!"
	}
}

type ChannelUpdateEvent struct {
	BaseEvent
	ChannelId   nocan.ChannelId
	ChannelName string
	Status      ChannelStatus
	Value       []byte
}

func (cu *ChannelUpdateEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Id     nocan.ChannelId `json:"id"`
		Name   string          `json:"name"`
		Status string          `json:"status"`
		Value  string          `json:"value"`
	}{
		Id:     cu.ChannelId,
		Name:   cu.ChannelName,
		Status: cu.Status.String(),
		Value:  string(cu.Value),
	})
}

func NewChannelUpdateEvent(chan_name string, chan_id nocan.ChannelId, status ChannelStatus, value []byte) *ChannelUpdateEvent {
	var v []byte

	if value != nil {
		v = make([]byte, len(value))
		copy(v, value)
	} else {
		v = nil
	}
	return &ChannelUpdateEvent{BaseEvent: BaseEvent{0, ChannelUpdateEventId}, ChannelId: chan_id, ChannelName: chan_name, Status: status, Value: v}
}

func (cu *ChannelUpdateEvent) PackedLength() int {
	return len(cu.ChannelName) + len(cu.Value) + 5
}

func (cu *ChannelUpdateEvent) Pack() ([]byte, error) {
	buf := make([]byte, 0, cu.PackedLength())
	buf = append(buf, byte(cu.Status))
	buf = append(buf, byte(cu.ChannelId>>8), byte(cu.ChannelId&0xFF))
	buf = append(buf, byte(len(cu.ChannelName)))
	buf = append(buf, []byte(cu.ChannelName)...)
	buf = append(buf, byte(len(cu.Value)))
	if cu.Value != nil {
		buf = append(buf, cu.Value...)
	}
	return buf, nil
}

func (cu *ChannelUpdateEvent) Unpack(value []byte) error {
	if len(value) < 3 {
		return ErrorMissingData
	}
	cu.Status = ChannelStatus(value[0])

	cu.ChannelId = (nocan.ChannelId(value[1]) << 8) | nocan.ChannelId(value[2])

	lName := int(value[3])
	/*
		if lName == 0 {
			return errors.New("Empty channel name")
		}
	*/
	if lName > 64 {
		return errors.New("Channel name exceeds 64 bytes")
	}

	value = value[4:]
	if lName > len(value) {
		return ErrorMissingData
	}
	cu.ChannelName = string(value[:lName])
	value = value[lName:]

	lValue := int(value[0])
	value = value[1:]
	if lValue > len(value) {
		return ErrorMissingData
	}
	cu.Value = make([]byte, lValue)
	copy(cu.Value, value)
	return nil
}

func (cu ChannelUpdateEvent) String() string {
	s := cu.BaseEvent.String()
	switch cu.Status {
	case CHANNEL_CREATED:
		s += fmt.Sprintf("CREATED\t#%d\t%s", cu.ChannelId, cu.ChannelName)
		break
	case CHANNEL_DESTROYED:
		s += fmt.Sprintf("DESTROYED\t#%d\t%s", cu.ChannelId, cu.ChannelName)
		break
	case CHANNEL_UPDATED:
		s += fmt.Sprintf("UPDATED\t#%d\t%s\t%q", cu.ChannelId, cu.ChannelName, cu.Value)
		break
	case CHANNEL_NOT_FOUND:
		s += fmt.Sprintf("NOT_FOUND\t#%d\t%s", cu.ChannelId, cu.ChannelName)
		break
	default:
		s += "ERROR"
		break
	}
	return s
}

// ChannelListRequestEvent
//
//

type ChannelListRequestEvent struct {
	EmptyEvent
}

func NewChannelListRequestEvent() *ChannelListRequestEvent {
	return &ChannelListRequestEvent{EmptyEvent{BaseEvent{0, ChannelListRequestEventId}}}
}

// ChannelListEvent
//
//
type ChannelListEvent struct {
	BaseEvent `json:"-"`
	Channels  []*ChannelUpdateEvent `json:"channels"`
}

func NewChannelListEvent() *ChannelListEvent {
	return &ChannelListEvent{BaseEvent: BaseEvent{0, ChannelListEventId}, Channels: make([]*ChannelUpdateEvent, 0, 8)}
}

func (cl *ChannelListEvent) Append(cu *ChannelUpdateEvent) {
	cl.Channels = append(cl.Channels, cu)
}

func (cl *ChannelListEvent) Pack() ([]byte, error) {
	alloc := 0
	for _, cu := range cl.Channels {
		alloc += cu.PackedLength()
	}
	b := make([]byte, 0, alloc)
	for _, cu := range cl.Channels {
		packed, err := cu.Pack()
		if err == nil {
			b = append(b, packed...)
		} else {
			return nil, err
		}
	}
	return b, nil
}

func (cl *ChannelListEvent) Unpack(b []byte) error {
	cl.Channels = make([]*ChannelUpdateEvent, 0, 8)

	for {
		if len(b) == 0 {
			break
		}
		cu := new(ChannelUpdateEvent)
		err := cu.Unpack(b)
		if err != nil {
			return err
		}
		cl.Append(cu)
		b = b[cu.PackedLength():]
	}
	return nil
}

func (cl ChannelListEvent) String() string {
	var resp string
	for _, cu := range cl.Channels {
		resp += cu.String() + "\n"
	}
	return resp
}

// NodeUpdateRequestEvent
//
//

type NodeUpdateRequestEvent struct {
	BaseEvent
	NodeId nocan.NodeId
}

func NewNodeUpdateRequestEvent(node_id nocan.NodeId) *NodeUpdateRequestEvent {
	return &NodeUpdateRequestEvent{BaseEvent: BaseEvent{0, NodeUpdateRequestEventId}, NodeId: node_id}
}

func (nu NodeUpdateRequestEvent) Pack() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(nu.NodeId)
	return b, nil
}

func (nu *NodeUpdateRequestEvent) Unpack(b []byte) error {
	if len(b) < 1 {
		return ErrorMissingData
	}
	nu.NodeId = nocan.NodeId(b[0])
	return nil
}

func (nu NodeUpdateRequestEvent) String() string {
	return fmt.Sprintf("#%d", nu.NodeId)
}

// NodeUpdateEvent
//
//

type NodeUpdateEvent struct {
	BaseEvent `json:-"`
	NodeId    nocan.NodeId     `json:"id"`
	State     models.NodeState `json:"state"`
	Udid      models.Udid8     `json:"udid"`
	LastSeen  time.Time        `json:"last_seen"`
}

func NewNodeUpdateEvent(id nocan.NodeId, state models.NodeState, udid models.Udid8, last_seen time.Time) *NodeUpdateEvent {
	nu := &NodeUpdateEvent{BaseEvent: BaseEvent{0, NodeUpdateEventId}, NodeId: id, State: state, LastSeen: last_seen.UTC()}
	copy(nu.Udid[:], udid[:])
	return nu
}

func (nu *NodeUpdateEvent) Pack() ([]byte, error) {
	b := make([]byte, 18)
	b[0] = byte(nu.NodeId)
	b[1] = byte(nu.State)
	copy(b[2:10], nu.Udid[:])
	EncodeUint64(b[10:18], uint64(nu.LastSeen.UnixNano()))
	return b, nil
}

func (nu *NodeUpdateEvent) Unpack(b []byte) error {
	if len(b) < 18 {
		return ErrorMissingData
	}
	nu.NodeId = nocan.NodeId(b[0])
	nu.State = models.NodeState(b[1])
	copy(nu.Udid[:], b[2:10])
	tm := DecodeUint64(b[10:18])
	nu.LastSeen = time.Unix(0, int64(tm)).UTC()
	return nil
}

func (nu NodeUpdateEvent) String() string {
	return fmt.Sprintf("#%d\t%s\t%s\t%s", nu.Id, nu.Udid, nu.State, nu.LastSeen.Format(time.RFC3339Nano))
}

// NodeListEvent
//
//

type NodeListEvent struct {
	BaseEvent
	Nodes []*NodeUpdateEvent `json:"nodes"`
}

func NewNodeListEvent() *NodeListEvent {
	return &NodeListEvent{BaseEvent: BaseEvent{0, NodeListEventId}, Nodes: make([]*NodeUpdateEvent, 0, 8)}
}

func (nl *NodeListEvent) Append(nu *NodeUpdateEvent) {
	nl.Nodes = append(nl.Nodes, nu)
}

func (nl *NodeListEvent) Pack() ([]byte, error) {
	b := make([]byte, 0, len(nl.Nodes)*18)
	for _, nu := range nl.Nodes {
		sb, _ := nu.Pack()
		b = append(b, sb...)
	}
	return b, nil
}

func (nl *NodeListEvent) Unpack(b []byte) error {
	nl.Nodes = make([]*NodeUpdateEvent, 0, 8)
	for {
		if len(b) == 0 {
			break
		}
		nu := new(NodeUpdateEvent)
		if err := nu.Unpack(b); err != nil {
			return err
		}
		nl.Append(nu)
		b = b[18:]
	}
	return nil
}

func (nl NodeListEvent) String() string {
	var resp string
	for _, nu := range nl.Nodes {
		resp += nu.String() + "\n"
	}
	return resp
}

// NodeListRequestEvent
//
//

type NodeListRequestEvent struct {
	EmptyEvent
}

func NewNodeListRequestEvent() *NodeListRequestEvent {
	return &NodeListRequestEvent{EmptyEvent{BaseEvent{0, NodeListRequestEventId}}}
}

// NodeFirmwareDownloadRequestEvent
//
//

type NodeFirmwareDownloadRequestEvent struct {
	BaseEvent
	NodeId nocan.NodeId
}

func NewNodeFirmwareDownloadRequestEvent(nid nocan.NodeId) *NodeFirmwareDownloadRequestEvent {
	return &NodeFirmwareDownloadRequestEvent{BaseEvent: BaseEvent{0, NodeFirmwareDownloadRequestEventId}, NodeId: nid}
}

func (ndr NodeFirmwareDownloadRequestEvent) Pack() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(ndr.NodeId)
	return b, nil
}

func (ndr *NodeFirmwareDownloadRequestEvent) Unpack(b []byte) error {
	if len(b) == 0 {
		return ErrorMissingData
	}
	ndr.NodeId = nocan.NodeId(b[0])
	return nil
}

// FirmwareBlock
//
//

type FirmwareBlock struct {
	Offset uint32
	Data   []byte
}

type NodeFirmwareEvent struct {
	BaseEvent
	NodeId   nocan.NodeId
	Download bool
	Limit    uint32
	Code     []FirmwareBlock
}

func NewNodeFirmwareEvent(id nocan.NodeId) *NodeFirmwareEvent {
	return &NodeFirmwareEvent{BaseEvent: BaseEvent{0, NodeFirmwareEventId}, NodeId: id, Limit: 0, Code: make([]FirmwareBlock, 0, 8)}
}

func (nf *NodeFirmwareEvent) ConfigureAsDownload() *NodeFirmwareEvent {
	nf.Download = true
	return nf
}

func (nf *NodeFirmwareEvent) ConfigureAsUpload() *NodeFirmwareEvent {
	nf.Download = false
	return nf
}

func (nf *NodeFirmwareEvent) AppendBlock(offset uint32, data []byte) {
	fb := FirmwareBlock{Offset: offset, Data: make([]byte, len(data))}
	copy(fb.Data, data)
	nf.Code = append(nf.Code, fb)
}

func (nf *NodeFirmwareEvent) Pack() ([]byte, error) {
	tlen := 6
	for _, block := range nf.Code {
		tlen += 8 + len(block.Data)
	}
	b := make([]byte, tlen)
	b[0] = byte(nf.NodeId)
	if nf.Download {
		b[1] = 1
	} else {
		b[1] = 0
	}

	EncodeUint32(b[2:], nf.Limit)

	pos := 6
	for _, block := range nf.Code {
		EncodeUint32(b[pos:], block.Offset)
		EncodeUint32(b[pos+4:], uint32(len(block.Data)))
		copy(b[pos+8:], block.Data)
		pos += 8 + len(block.Data)
	}
	return b, nil
}

func (nf *NodeFirmwareEvent) Unpack(b []byte) error {
	if len(b) < 2 {
		return ErrorMissingData
	}
	nf.NodeId = nocan.NodeId(b[0])
	if b[1] == 0 {
		nf.Download = false
	} else {
		nf.Download = true
	}

	nf.Limit = DecodeUint32(b[2:])

	b = b[6:]
	for {
		if len(b) == 0 {
			break
		}
		if len(b) < 8 {
			return ErrorMissingData
		}
		offset := DecodeUint32(b)
		blen := DecodeUint32(b[4:])
		b = b[8:]
		if len(b) < int(blen) {
			return ErrorMissingData
		}
		nf.AppendBlock(offset, b[:blen])
		b = b[blen:]
	}
	return nil
}

//
//
//

type ProgressReport byte

const (
	ProgressSuccess ProgressReport = 0xFE
	ProgressFailed  ProgressReport = 0xFF
)

func (pr ProgressReport) String() string {
	if pr <= 100 {
		return fmt.Sprintf("%d%%", pr)
	}
	switch pr {
	case ProgressSuccess:
		return "Success"
	case ProgressFailed:
		return "Failed"
	}
	return "!unknown!"
}

type NodeFirmwareProgressEvent struct {
	BaseEvent
	NodeId           nocan.NodeId
	Progress         ProgressReport
	BytesTransferred uint32
}

func NewNodeFirmwareProgressEvent(id nocan.NodeId) *NodeFirmwareProgressEvent {
	return &NodeFirmwareProgressEvent{BaseEvent: BaseEvent{0, NodeFirmwareProgressEventId}, NodeId: id, Progress: 0, BytesTransferred: 0}
}

func (nfp *NodeFirmwareProgressEvent) Update(progress ProgressReport, transferred uint32) *NodeFirmwareProgressEvent {
	return &NodeFirmwareProgressEvent{BaseEvent: BaseEvent{0, NodeFirmwareProgressEventId}, NodeId: nfp.NodeId, Progress: progress, BytesTransferred: transferred}
}

func (nfp *NodeFirmwareProgressEvent) MarkAsFailed() *NodeFirmwareProgressEvent {
	return nfp.Update(ProgressFailed, 0)
}

func (nfp *NodeFirmwareProgressEvent) MarkAsSuccess() *NodeFirmwareProgressEvent {
	return nfp.Update(ProgressSuccess, 0)
}

func (nfp *NodeFirmwareProgressEvent) Pack() ([]byte, error) {
	b := make([]byte, 6)
	b[0] = byte(nfp.NodeId)
	b[1] = byte(nfp.Progress)
	EncodeUint32(b[2:], nfp.BytesTransferred)
	return b, nil
}

func (nfp *NodeFirmwareProgressEvent) Unpack(b []byte) error {
	if len(b) < 6 {
		return ErrorMissingData
	}
	nfp.NodeId = nocan.NodeId(b[0])
	nfp.Progress = ProgressReport(b[1])
	nfp.BytesTransferred = DecodeUint32(b[2:])
	return nil
}

func (nfp NodeFirmwareProgressEvent) String() string {
	return nfp.BaseEvent.String() + nfp.Progress.String()
}

// BusPowerEvent
//
//

type BusPowerEvent struct {
	BaseEvent
	PowerOn bool
}

func NewBusPowerEvent(power_on bool) *BusPowerEvent {
	return &BusPowerEvent{BaseEvent: BaseEvent{0, BusPowerEventId}, PowerOn: power_on}
}

func (bp BusPowerEvent) Pack() ([]byte, error) {
	b := make([]byte, 1)
	if bp.PowerOn {
		b[0] = 1
	} else {
		b[0] = 0
	}
	return b, nil
}

func (bp *BusPowerEvent) Unpack(b []byte) error {
	if len(b) != 1 {
		return ErrorMissingData
	}
	bp.PowerOn = (b[0] == 0)
	return nil
}

//
//
//

type BusPowerStatusUpdateRequestEvent struct {
	EmptyEvent
}

func NewBusPowerStatusUpdateRequestEvent() *BusPowerStatusUpdateRequestEvent {
	return &BusPowerStatusUpdateRequestEvent{EmptyEvent{BaseEvent{0, BusPowerStatusUpdateRequestEventId}}}
}

//
//
//

type DeviceInformationRequestEvent struct {
	EmptyEvent
}

func NewDeviceInformationRequestEvent() *DeviceInformationRequestEvent {
	return &DeviceInformationRequestEvent{EmptyEvent{BaseEvent{0, DeviceInformationRequestEventId}}}
}

//
//
//

type SystemPropertiesRequestEvent struct {
	EmptyEvent
}

func NewSystemPropertiesRequestEvent() *SystemPropertiesRequestEvent {
	return &SystemPropertiesRequestEvent{EmptyEvent{BaseEvent{0, SystemPropertiesRequestEventId}}}
}

//
//
//

type NodeRebootRequestEvent struct {
	BaseEvent
	RebootId byte
}

func NewNodeRebootRequestEvent(nid nocan.NodeId, force bool) *NodeRebootRequestEvent {
	rid := byte(nid)
	if force {
		rid |= 128
	}
	return &NodeRebootRequestEvent{BaseEvent: BaseEvent{0, NodeRebootRequestEventId}, RebootId: rid}
}

func (nr NodeRebootRequestEvent) NodeId() nocan.NodeId {
	return nocan.NodeId(nr.RebootId & 0x7F)
}

func (nr *NodeRebootRequestEvent) Forced() bool {
	return (nr.RebootId & 128) != 0
}

func (nr NodeRebootRequestEvent) Pack() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = nr.RebootId
	return b, nil
}

func (nr *NodeRebootRequestEvent) Unpack(b []byte) error {
	if len(b) != 1 {
		return ErrorMissingData
	}
	nr.RebootId = b[0]
	return nil
}

func (nr NodeRebootRequestEvent) String() string {
	return nr.BaseEvent.String() + fmt.Sprintf("%d %t", nr.NodeId(), nr.Forced())
}

//
//
//

type DeviceInformationEvent struct {
	BaseEvent
	Information *device.Information
}

func NewDeviceInformationEvent(di *device.Information) *DeviceInformationEvent {
	return &DeviceInformationEvent{BaseEvent: BaseEvent{0, DeviceInformationEventId}, Information: di}
}

func (ie DeviceInformationEvent) Pack() ([]byte, error) {
	buf := make([]byte, 0, 26)
	buf = append(buf, ie.Information.Type[:]...)
	buf = append(buf, ie.Information.Signature[:]...)
	buf = append(buf, ie.Information.VersionMajor)
	buf = append(buf, ie.Information.VersionMinor)
	buf = append(buf, ie.Information.ChipId[:]...)
	return buf, nil
}

func (ie *DeviceInformationEvent) Unpack(b []byte) error {
	if len(b) < 26 {
		fmt.Errorf("Device info must be at least 18 bytes long, found %d", len(b))
	}
	di := new(device.Information)
	copy(di.Type[:], b[0:8])
	copy(di.Signature[:], b[8:12])
	di.VersionMajor = b[12]
	di.VersionMinor = b[13]
	copy(di.ChipId[:], b[14:26])
	ie.Information = di
	return nil
}

func (ie DeviceInformationEvent) String() string {
	return ie.BaseEvent.String() + ie.Information.String()
}

//
//
//

type SystemPropertiesEvent struct {
	BaseEvent
	Properties *properties.Properties
}

func NewSystemPropertiesEvent(props *properties.Properties) *SystemPropertiesEvent {
	if props == nil {
		props = properties.New()
	}
	return &SystemPropertiesEvent{BaseEvent: BaseEvent{0, SystemPropertiesEventId}, Properties: props}
}

func (sp SystemPropertiesEvent) Pack() ([]byte, error) {
	buf := new(bytes.Buffer)

	for key, prop := range sp.Properties.Map {
		buf.WriteByte(byte(len(key)))
		buf.WriteString(key)

		buf.WriteByte(prop.Type)
		switch prop.Type {
		case properties.PROP_TYPE_NULL:
			// do nothing
		case properties.PROP_TYPE_INT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(int32))
		case properties.PROP_TYPE_UINT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(uint32))
		case properties.PROP_TYPE_FLOAT32:
			binary.Write(buf, binary.BigEndian, prop.Value.(uint32))
		case properties.PROP_TYPE_BOOL:
			binary.Write(buf, binary.BigEndian, prop.Value.(bool))
		case properties.PROP_TYPE_STRING:
			s := prop.Value.(string)
			buf.WriteByte(byte(len(s)))
			buf.WriteString(s)
		default:
			return nil, fmt.Errorf("Unexpected type %d while packing Property", prop.Type)
		}
	}
	return buf.Bytes(), nil
}

func (sp *SystemPropertiesEvent) Unpack(b []byte) error {
	var len byte
	var ptype byte
	var err error

	sp.Properties = properties.New()

	buf := bytes.NewReader(b)

	for buf.Len() > 0 {
		if len, err = buf.ReadByte(); err != nil {
			return err
		}
		key := make([]byte, len)
		if _, err := buf.Read(key); err != nil {
			return err
		}
		if ptype, err = buf.ReadByte(); err != nil {
			return err
		}
		switch ptype {
		case properties.PROP_TYPE_INT32:
			var i int32
			binary.Read(buf, binary.BigEndian, &i)
			sp.Properties.AddInt32(string(key), i)
		case properties.PROP_TYPE_UINT32:
			var u uint32
			binary.Read(buf, binary.BigEndian, &u)
			sp.Properties.AddUint32(string(key), u)
		case properties.PROP_TYPE_FLOAT32:
			var f float32
			binary.Read(buf, binary.BigEndian, &f)
			sp.Properties.AddFloat32(string(key), f)
		case properties.PROP_TYPE_BOOL:
			var b bool
			binary.Read(buf, binary.BigEndian, &b)
			sp.Properties.AddBool(string(key), b)
		case properties.PROP_TYPE_STRING:
			var slen byte
			if slen, err = buf.ReadByte(); err != nil {
				return err
			}
			sval := make([]byte, slen)
			if _, err := buf.Read(sval); err != nil {
				return err
			}
			sp.Properties.AddString(string(key), string(sval))
		default:
			return fmt.Errorf("Unexpected type %d unpacking property '%s'", ptype, string(key))
		}
	}
	return nil
}

func (sp SystemPropertiesEvent) String() string {
	return sp.BaseEvent.String() + fmt.Sprintf("[%d properties]", len(sp.Properties.Map))
}

//
//
//

type BusPowerStatusUpdateEvent struct {
	BaseEvent
	Status *device.PowerStatus
}

func NewBusPowerStatusUpdateEvent(status *device.PowerStatus) *BusPowerStatusUpdateEvent {
	return &BusPowerStatusUpdateEvent{BaseEvent: BaseEvent{0, BusPowerStatusUpdateEventId}, Status: status}
}

func (bps BusPowerStatusUpdateEvent) Pack() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, bps.Status)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (bps *BusPowerStatusUpdateEvent) Unpack(b []byte) error {
	buf := bytes.NewReader(b)
	status := new(device.PowerStatus)
	err := binary.Read(buf, binary.BigEndian, status)
	if err != nil {
		return err
	}
	bps.Status = status
	return nil
}

func (bps BusPowerStatusUpdateEvent) String() string {
	return bps.BaseEvent.String() + fmt.Sprintf("Driver voltage=%.1f, current sense=%d, reference voltage=%.2f, status(%x)=%s.",
		bps.Status.Voltage,
		bps.Status.CurrentSense,
		bps.Status.RefLevel,
		byte(bps.Status.Status), bps.Status.Status)
}

/****** *******/

const (
	NoEventId                          EventId = 0
	ClientHelloEventId                         = 1
	ServerHelloEventId                         = 2
	ChannelFilterEventId                       = 3
	ServerAckEventId                           = 4
	Reserved5EventId                           = 5
	BusPowerStatusUpdateEventId                = 6
	BusPowerEventId                            = 7
	ChannelUpdateRequestEventId                = 8
	ChannelUpdateEventId                       = 9
	ChannelListRequestEventId                  = 10
	ChannelListEventId                         = 11
	NodeUpdateRequestEventId                   = 12
	NodeUpdateEventId                          = 13
	NodeListRequestEventId                     = 14
	NodeListEventId                            = 15
	NodeFirmwareEventId                        = 16
	NodeFirmwareDownloadRequestEventId         = 17
	ReservedEvent16EventId                     = 18
	NodeFirmwareProgressEventId                = 19
	NodeRebootRequestEventId                   = 20
	BusPowerStatusUpdateRequestEventId         = 21
	DeviceInformationRequestEventId            = 22
	DeviceInformationEventId                   = 23
	SystemPropertiesRequestEventId             = 24
	SystemPropertiesEventId                    = 25
	EventIdCount                               = 26
)

var EventNames = [EventIdCount]string{
	"no-event",
	"client-hello-event",
	"server-hello-event",
	"channel-subscribe-event",
	"server-ack-event",
	"reserved5-event",
	"bus-power-status-update-event",
	"bus-power-event",
	"channel-update-request-event",
	"channel-update-event",
	"channel-list-request-event",
	"channel-list-event",
	"node-update-request-event",
	"node-update-event",
	"node-list-request-event",
	"node-list-event",
	"node-firmware-upload-event",
	"node-firmware-download-request-event",
	"node-firmware-download-event",
	"node-firmware-progress-event",
	"node-reboot-request-event",
	"bus-power-status-update-request-event",
	"device-information-request-event",
	"device-information-event",
	"system-properties-request-event",
	"system-properties-event",
}

var EventNameMap map[string]EventId

func LookupEventByName(name string) EventId {
	return EventNameMap[name]
}

func (eid EventId) String() string {
	if eid < EventIdCount {
		return EventNames[eid]
	}
	return "unknown-event"
}

func init() {
	EventNameMap = make(map[string]EventId)
	for i, n := range EventNames {
		EventNameMap[n] = EventId(i)
	}
}
