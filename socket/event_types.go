package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/nocan"
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

type EmptyRequest struct{}

func (x EmptyRequest) Pack() ([]byte, error) {
	return make([]byte, 0), nil
}

func (c *EmptyRequest) Unpack(b []byte) error {
	return nil
}

/****************************************************************************/

type ClientHello struct {
	Tool         string
	VersionMajor byte
	VersionMinor byte
}

func NewClientHello(tool string, major byte, minor byte) *ClientHello {
	return &ClientHello{tool, major, minor}
}

func (ch *ClientHello) Pack() ([]byte, error) {
	b := make([]byte, len(ch.Tool)+3, 0)
	b = append(b, byte(len(ch.Tool)))
	b = append(b, []byte(ch.Tool)...)
	b = append(b, ch.VersionMajor)
	b = append(b, ch.VersionMinor)
	return b, nil
}

func (ch *ClientHello) Unpack(b []byte) error {
	if int(b[0]+2) > len(b) {
		return ErrorMissingData
	}
	ch.Tool = string(b[1 : 1+b[0]])
	ch.VersionMajor = b[b[0]]
	ch.VersionMinor = b[b[0]+1]
	return nil
}

/****************************************************************************/

type ServerHello struct {
	ClientHello
}

func NewServerHello(tool string, major byte, minor byte) *ServerHello {
	return &ServerHello{ClientHello{tool, major, minor}}
}

/****************************************************************************/
// ChannelSubscriptionList represents the list of channels that a client will receive.
//

type ChannelSubscriptionList struct {
	Channels map[nocan.ChannelId]bool
}

func NewChannelSubscriptionList(chans ...nocan.ChannelId) *ChannelSubscriptionList {
	clist := &ChannelSubscriptionList{Channels: make(map[nocan.ChannelId]bool)}
	clist.Add(chans...)
	return clist
}

func (sl *ChannelSubscriptionList) Add(chans ...nocan.ChannelId) {
	for _, e := range chans {
		sl.Channels[e] = true
	}
}

func (sl *ChannelSubscriptionList) Remove(chans ...nocan.ChannelId) {
	for _, e := range chans {
		delete(sl.Channels, e)
	}
}

func (sl *ChannelSubscriptionList) Includes(id nocan.ChannelId) bool {
	return sl.Channels[id]
}

func (sl *ChannelSubscriptionList) Pack() ([]byte, error) {
	p := make([]byte, len(sl.Channels)*2)

	i := 0
	for k, _ := range sl.Channels {
		EncodeUint16(p[i:], uint16(k))
		i += 2
	}
	return p, nil
}

func (sl *ChannelSubscriptionList) Unpack(b []byte) error {
	for i := 0; i < len(b); i += 2 {
		x := nocan.ChannelId(DecodeUint16(b[i:]))
		sl.Add(x)
	}
	return nil
}

func (sl *ChannelSubscriptionList) String() string {
	s := "["

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

type ServerAck struct {
	Code byte
}

var (
	ServerAckSuccess        = NewServerAck(0)
	ServerAckBadRequest     = NewServerAck(1)
	ServerAckUnauthorized   = NewServerAck(2)
	ServerAckNotFound       = NewServerAck(3)
	ServerAckGeneralFailure = NewServerAck(4)
	ServerAckUnknown        = NewServerAck(5)
)

var serverAckStrings = [5]string{
	"Success",
	"Bad request",
	"Unauthorized",
	"Not found",
	"General failure",
}

var (
	ErrorServerAckBadRequest     = errors.New("Bad request")
	ErrorServerAckUnauthorized   = errors.New("Unauthorized")
	ErrorServerAckNotFound       = errors.New("Not found")
	ErrorServerAckGeneralFailure = errors.New("General failure")
	ErrorServerAckUnknown        = errors.New("Unknown error")
)

func NewServerAck(val byte) *ServerAck {
	return &ServerAck{val}
}

func (sa *ServerAck) Pack() ([]byte, error) {
	p := make([]byte, 1)
	p[0] = sa.Code
	return p, nil
}

func (sa *ServerAck) Unpack(value []byte) error {
	if len(value) < 1 {
		return ErrorMissingData
	}
	sa.Code = value[0]
	return nil
}

func (sa ServerAck) String() string {
	if sa.Code < 5 {
		return serverAckStrings[sa.Code]
	}
	return "Unknown error"
}

func (sa ServerAck) ToError() error {
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
	}
	return ErrorServerAckUnknown
}

/****************************************************************************/

// ChannelUpdateRequest
//
//

type ChannelUpdateRequest struct {
	Id   nocan.ChannelId
	Name string
}

func NewChannelUpdateRequest(chan_name string, chan_id nocan.ChannelId) *ChannelUpdateRequest {
	return &ChannelUpdateRequest{chan_id, chan_name}
}

func (cu *ChannelUpdateRequest) PackedLength() int {
	return len(cu.Name) + 3
}

func (cu *ChannelUpdateRequest) Pack() ([]byte, error) {
	buf := make([]byte, 0, cu.PackedLength())
	buf = append(buf, byte(cu.Id>>8), byte(cu.Id&0xFF))
	buf = append(buf, byte(len(cu.Name)))
	buf = append(buf, []byte(cu.Name)...)
	return buf, nil
}

func (cu *ChannelUpdateRequest) Unpack(value []byte) error {
	if len(value) < 3 {
		return ErrorMissingData
	}
	cu.Id = (nocan.ChannelId(value[0]) << 8) | nocan.ChannelId(value[1])

	lName := int(value[2])
	if lName > 64 {
		return errors.New("Channel name exceeds 64 bytes")
	}
	value = value[3:]
	if lName > len(value) {
		return ErrorMissingData
	}
	cu.Name = string(value[:lName])
	return nil
}

func (cu ChannelUpdateRequest) String() string {
	if cu.Name == "" {
		return fmt.Sprintf("#%d", cu.Id)
	}
	return fmt.Sprintf("%s", cu.Name)
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

type ChannelUpdate struct {
	Id     nocan.ChannelId
	Name   string
	Status ChannelStatus
	Value  []byte
}

func (cu *ChannelUpdate) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Id     nocan.ChannelId `json:"id"`
		Name   string          `json:"name"`
		Status string          `json:"status"`
		Value  string          `json:"value"`
	}{
		Id:     cu.Id,
		Name:   cu.Name,
		Status: cu.Status.String(),
		Value:  string(cu.Value),
	})
}

func NewChannelUpdate(chan_name string, chan_id nocan.ChannelId, status ChannelStatus, value []byte) *ChannelUpdate {
	var v []byte

	if value != nil {
		v = make([]byte, len(value))
		copy(v, value)
	} else {
		v = nil
	}
	return &ChannelUpdate{chan_id, chan_name, status, v}
}

func (cu *ChannelUpdate) PackedLength() int {
	return len(cu.Name) + len(cu.Value) + 5
}

func (cu *ChannelUpdate) Pack() ([]byte, error) {
	buf := make([]byte, 0, cu.PackedLength())
	buf = append(buf, byte(cu.Status))
	buf = append(buf, byte(cu.Id>>8), byte(cu.Id&0xFF))
	buf = append(buf, byte(len(cu.Name)))
	buf = append(buf, []byte(cu.Name)...)
	buf = append(buf, byte(len(cu.Value)))
	if cu.Value != nil {
		buf = append(buf, cu.Value...)
	}
	return buf, nil
}

func (cu *ChannelUpdate) Unpack(value []byte) error {
	if len(value) < 3 {
		return ErrorMissingData
	}
	cu.Status = ChannelStatus(value[0])

	cu.Id = (nocan.ChannelId(value[1]) << 8) | nocan.ChannelId(value[2])

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
	cu.Name = string(value[:lName])
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

func (cu ChannelUpdate) String() string {
	switch cu.Status {
	case CHANNEL_CREATED:
		return fmt.Sprintf("CREATED\t#%d\t%s", cu.Id, cu.Name)
	case CHANNEL_DESTROYED:
		return fmt.Sprintf("DESTROYED\t#%d\t%s", cu.Id, cu.Name)
	case CHANNEL_UPDATED:
		return fmt.Sprintf("UPDATED\t#%d\t%s\t%q", cu.Id, cu.Name, cu.Value)
	case CHANNEL_NOT_FOUND:
		return fmt.Sprintf("NOT_FOUND\t#%d\t%s", cu.Id, cu.Name)
	}
	return "ERROR"
}

// ChannelListRequest
//
//

type ChannelListRequest struct {
	EmptyRequest
}

// ChannelList
//
//
type ChannelList struct {
	Channels []*ChannelUpdate `json:"channels"`
}

func NewChannelList() *ChannelList {
	return &ChannelList{make([]*ChannelUpdate, 0, 8)}
}

func (cl *ChannelList) Append(cu *ChannelUpdate) {
	cl.Channels = append(cl.Channels, cu)
}

func (cl *ChannelList) Pack() ([]byte, error) {
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

func (cl *ChannelList) Unpack(b []byte) error {
	cl.Channels = make([]*ChannelUpdate, 0, 8)

	for {
		if len(b) == 0 {
			break
		}
		cu := new(ChannelUpdate)
		err := cu.Unpack(b)
		if err != nil {
			return err
		}
		cl.Append(cu)
		b = b[cu.PackedLength():]
	}
	return nil
}

func (cl ChannelList) String() string {
	var resp string
	for _, cu := range cl.Channels {
		resp += cu.String() + "\n"
	}
	return resp
}

// NodeUpdateRequest
//
//

type NodeUpdateRequest struct {
	NodeId nocan.NodeId
}

func NewNodeUpdateRequest(node_id nocan.NodeId) *NodeUpdateRequest {
	return &NodeUpdateRequest{node_id}
}

func (nu NodeUpdateRequest) Pack() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(nu.NodeId)
	return b, nil
}

func (nu *NodeUpdateRequest) Unpack(b []byte) error {
	if len(b) < 1 {
		return ErrorMissingData
	}
	nu.NodeId = nocan.NodeId(b[0])
	return nil
}

func (nu NodeUpdateRequest) String() string {
	return fmt.Sprintf("#%d", nu.NodeId)
}

// NodeUpdate
//
//

type NodeUpdate struct {
	Id       nocan.NodeId     `json:"id"`
	State    models.NodeState `json:"state"`
	Udid     models.Udid8     `json:"udid"`
	LastSeen time.Time        `json:"last_seen"`
}

func NewNodeUpdate(id nocan.NodeId, state models.NodeState, udid models.Udid8, last_seen time.Time) *NodeUpdate {
	nu := &NodeUpdate{Id: id, State: state, LastSeen: last_seen.UTC()}
	copy(nu.Udid[:], udid[:])
	return nu
}

func (nu *NodeUpdate) Pack() ([]byte, error) {
	b := make([]byte, 18)
	b[0] = byte(nu.Id)
	b[1] = byte(nu.State)
	copy(b[2:10], nu.Udid[:])
	EncodeUint64(b[10:18], uint64(nu.LastSeen.UnixNano()))
	return b, nil
}

func (nu *NodeUpdate) Unpack(b []byte) error {
	if len(b) < 18 {
		return ErrorMissingData
	}
	nu.Id = nocan.NodeId(b[0])
	nu.State = models.NodeState(b[1])
	copy(nu.Udid[:], b[2:10])
	tm := DecodeUint64(b[10:18])
	nu.LastSeen = time.Unix(0, int64(tm)).UTC()
	return nil
}

func (nu NodeUpdate) String() string {
	return fmt.Sprintf("#%d\t%s\t%s\t%s", nu.Id, nu.Udid, nu.State, nu.LastSeen.Format(time.RFC3339Nano))
}

// NodeList
//
//

type NodeList struct {
	Nodes []*NodeUpdate `json:"nodes"`
}

func NewNodeList() *NodeList {
	return &NodeList{Nodes: make([]*NodeUpdate, 0, 8)}
}

func (nl *NodeList) Append(nu *NodeUpdate) {
	nl.Nodes = append(nl.Nodes, nu)
}

func (nl *NodeList) Pack() ([]byte, error) {
	b := make([]byte, 0, len(nl.Nodes)*18)
	for _, nu := range nl.Nodes {
		sb, _ := nu.Pack()
		b = append(b, sb...)
	}
	return b, nil
}

func (nl *NodeList) Unpack(b []byte) error {
	nl.Nodes = make([]*NodeUpdate, 0, 8)
	for {
		if len(b) == 0 {
			break
		}
		nu := &NodeUpdate{}
		if err := nu.Unpack(b); err != nil {
			return err
		}
		nl.Append(nu)
		b = b[18:]
	}
	return nil
}

func (nl NodeList) String() string {
	var resp string
	for _, nu := range nl.Nodes {
		resp += nu.String() + "\n"
	}
	return resp
}

// NodeListRequest
//
//

type NodeListRequest struct {
	EmptyRequest
}

// NodeFirmwareDownloadRequest
//
//

type NodeFirmwareDownloadRequest struct {
	EmptyRequest
}

// FirmwareBlock
//
//

type FirmwareBlock struct {
	Offset uint32
	Data   []byte
}

type NodeFirmware struct {
	Id       nocan.NodeId
	Download bool
	Limit    uint32
	Code     []FirmwareBlock
}

//const MAX_UINT32 = (1 << 32) - 1

func NewNodeFirmware(id nocan.NodeId, isDownload bool) *NodeFirmware {
	return &NodeFirmware{Id: id, Download: isDownload, Limit: 0, Code: make([]FirmwareBlock, 0, 8)}
}

func (nf *NodeFirmware) AppendBlock(offset uint32, data []byte) {
	fb := FirmwareBlock{Offset: offset, Data: make([]byte, len(data))}
	copy(fb.Data, data)
	nf.Code = append(nf.Code, fb)
}

func (nf *NodeFirmware) Pack() ([]byte, error) {
	tlen := 6
	for _, block := range nf.Code {
		tlen += 8 + len(block.Data)
	}
	b := make([]byte, tlen)
	b[0] = byte(nf.Id)
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

func (nf *NodeFirmware) Unpack(b []byte) error {
	if len(b) < 2 {
		return ErrorMissingData
	}
	nf.Id = nocan.NodeId(b[0])
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

type NodeFirmwareProgress struct {
	Id               nocan.NodeId
	Progress         ProgressReport
	BytesTransferred uint32
}

func NewFirmwareProgress(id nocan.NodeId) *NodeFirmwareProgress {
	return &NodeFirmwareProgress{Id: id, Progress: 0, BytesTransferred: 0}
}

func (nfp *NodeFirmwareProgress) Update(progress ProgressReport, transferred uint32) *NodeFirmwareProgress {
	return &NodeFirmwareProgress{Id: nfp.Id, Progress: progress, BytesTransferred: transferred}
}

func (nfp *NodeFirmwareProgress) MarkAsFailed() *NodeFirmwareProgress {
	return nfp.Update(ProgressFailed, 0)
}

func (nfp *NodeFirmwareProgress) MarkAsSuccess() *NodeFirmwareProgress {
	return nfp.Update(ProgressSuccess, 0)
}

func (nfp *NodeFirmwareProgress) Pack() ([]byte, error) {
	b := make([]byte, 6)
	b[0] = byte(nfp.Id)
	b[1] = byte(nfp.Progress)
	EncodeUint32(b[2:], nfp.BytesTransferred)
	return b, nil
}

func (nfp *NodeFirmwareProgress) Unpack(b []byte) error {
	if len(b) < 6 {
		return ErrorMissingData
	}
	nfp.Id = nocan.NodeId(b[0])
	nfp.Progress = ProgressReport(b[1])
	nfp.BytesTransferred = DecodeUint32(b[2:])
	return nil
}

// BusPower
//
//

type BusPower struct {
	PowerOn bool
}

func NewBusPower(power_on bool) *BusPower {
	return &BusPower{power_on}
}

func (bp BusPower) Pack() ([]byte, error) {
	b := make([]byte, 1)
	if bp.PowerOn {
		b[0] = 1
	} else {
		b[0] = 0
	}
	return b, nil
}

func (bp *BusPower) Unpack(b []byte) error {
	if len(b) != 1 {
		return ErrorMissingData
	}
	bp.PowerOn = (b[0] == 0)
	return nil
}

//
//
//

type BusPowerStatusUpdateRequest struct {
	EmptyRequest
}

type DeviceInformationRequest struct {
	EmptyRequest
}

type SystemPropertiesRequest struct {
	EmptyRequest
}

//
//
//

type NodeRebootRequest byte

func CreateNodeRebootRequest(nid nocan.NodeId, force bool) NodeRebootRequest {
	if force {
		return NodeRebootRequest(nid) | 128
	}
	return NodeRebootRequest(nid)
}

func (nr NodeRebootRequest) NodeId() nocan.NodeId {
	return nocan.NodeId(nr & 0x7F)
}

func (nr NodeRebootRequest) Force() bool {
	return (nr & 128) != 0
}

func (nr NodeRebootRequest) Pack() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(nr)
	return b, nil
}

func (nr *NodeRebootRequest) Unpack(b []byte) error {
	if len(b) != 1 {
		return ErrorMissingData
	}
	*nr = NodeRebootRequest(b[0])
	return nil
}

/****** *******/

const (
	NoEvent                          EventId = 0
	ClientHelloEvent                         = 1
	ServerHelloEvent                         = 2
	ChannelSubscribeEvent                    = 3
	ServerAckEvent                           = 4
	Reserved5Event                           = 5
	BusPowerStatusUpdateEvent                = 6
	BusPowerEvent                            = 7
	ChannelUpdateRequestEvent                = 8
	ChannelUpdateEvent                       = 9
	ChannelListRequestEvent                  = 10
	ChannelListEvent                         = 11
	NodeUpdateRequestEvent                   = 12
	NodeUpdateEvent                          = 13
	NodeListRequestEvent                     = 14
	NodeListEvent                            = 15
	NodeFirmwareUploadEvent                  = 16
	NodeFirmwareDownloadRequestEvent         = 17
	NodeFirmwareDownloadEvent                = 18
	NodeFirmwareProgressEvent                = 19
	NodeRebootRequestEvent                   = 20
	BusPowerStatusUpdateRequestEvent         = 21
	DeviceInformationRequestEvent            = 22
	DeviceInformationEvent                   = 23
	SystemPropertiesRequestEvent             = 24
	SystemPropertiesEvent                    = 25
	EventCount                               = 26
)

var EventNames = [EventCount]string{
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
	"device-information--event",
	"system-properties-request-event",
	"system-properties-event",
}

var EventNameMap map[string]EventId

func LookupEventByName(name string) EventId {
	return EventNameMap[name]
}

func (eid EventId) String() string {
	if eid < EventCount {
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
