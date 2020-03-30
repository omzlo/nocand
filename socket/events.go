package socket

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/nocan"
	"time"
)

var ErrorMissingData error = errors.New("Missing data for value decoder")

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

// SubscriptionList represents the list of events that a client will receive.
// It also serves as the value of the ClientSubscriptionEvent, so it implements
// the ValuePacker interface
//
type SubscriptionList struct {
	Items map[EventId]bool
}

func NewSubscriptionList(subs ...EventId) *SubscriptionList {
	slist := &SubscriptionList{Items: make(map[EventId]bool)}
	slist.Add(subs...)
	return slist
}

func (sl *SubscriptionList) Add(subs ...EventId) {
	for _, e := range subs {
		sl.Items[e] = true
	}
}

func (sl *SubscriptionList) Remove(subs ...EventId) {
	for _, e := range subs {
		delete(sl.Items, e)
	}
}

func (sl *SubscriptionList) Includes(id EventId) bool {
	return sl.Items[id]
}

func (sl *SubscriptionList) PackValue() ([]byte, error) {
	p := make([]byte, len(sl.Items))

	i := 0
	for k, _ := range sl.Items {
		p[i] = byte(k)
		i++
	}
	return p, nil
}

func (sl *SubscriptionList) UnpackValue(b []byte) error {
	for _, x := range b {
		sl.Add(EventId(x))
	}
	return nil
}

/****************************************************************************/

// Authenticator
//
//
type Authenticator string

func (a Authenticator) PackValue() ([]byte, error) {
	return []byte(a), nil
}

func (a *Authenticator) UnpackValue(b []byte) error {
	*a = Authenticator(b)
	return nil
}

/****************************************************************************/

// ServerAck
//
//

type ServerAck byte

const (
	SERVER_ACK_SUCCESS ServerAck = iota
	SERVER_ACK_BAD_REQUEST
	SERVER_ACK_UNAUTHORIZED
	SERVER_ACK_NOT_FOUND
	SERVER_ACK_GENERAL_FAILURE
	SERVER_ACK_UNKNOWN
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

func (sa ServerAck) PackValue() ([]byte, error) {
	p := make([]byte, 1)
	p[0] = byte(sa)
	return p, nil
}

func (sa *ServerAck) UnpackValue(value []byte) error {
	if len(value) < 1 {
		return ErrorMissingData
	}
	*sa = ServerAck(value[0])
	return nil
}

func (sa ServerAck) String() string {
	if sa < 5 {
		return serverAckStrings[sa]
	}
	return "Unknown error"
}

func (sa ServerAck) ToError() error {
	switch sa {
	case SERVER_ACK_SUCCESS:
		return nil
	case SERVER_ACK_BAD_REQUEST:
		return ErrorServerAckBadRequest
	case SERVER_ACK_UNAUTHORIZED:
		return ErrorServerAckUnauthorized
	case SERVER_ACK_NOT_FOUND:
		return ErrorServerAckNotFound
	case SERVER_ACK_GENERAL_FAILURE:
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

func (cu *ChannelUpdateRequest) PackValue() ([]byte, error) {
	buf := make([]byte, 0, cu.PackedLength())
	buf = append(buf, byte(cu.Id>>8), byte(cu.Id&0xFF))
	buf = append(buf, byte(len(cu.Name)))
	buf = append(buf, []byte(cu.Name)...)
	return buf, nil
}

func (cu *ChannelUpdateRequest) UnpackValue(value []byte) error {
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

func (cu *ChannelUpdate) PackValue() ([]byte, error) {
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

func (cu *ChannelUpdate) UnpackValue(value []byte) error {
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

/* Channel list request is an empty event */

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

func (cl *ChannelList) PackValue() ([]byte, error) {
	alloc := 0
	for _, cu := range cl.Channels {
		alloc += cu.PackedLength()
	}
	b := make([]byte, 0, alloc)
	for _, cu := range cl.Channels {
		packed, err := cu.PackValue()
		if err == nil {
			b = append(b, packed...)
		} else {
			return nil, err
		}
	}
	return b, nil
}

func (cl *ChannelList) UnpackValue(b []byte) error {
	cl.Channels = make([]*ChannelUpdate, 0, 8)

	for {
		if len(b) == 0 {
			break
		}
		cu := &ChannelUpdate{}
		err := cu.UnpackValue(b)
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

type NodeUpdateRequest nocan.NodeId

func (nu NodeUpdateRequest) PackValue() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(nu)
	return b, nil
}

func (nu *NodeUpdateRequest) UnpackValue(b []byte) error {
	if len(b) < 1 {
		return ErrorMissingData
	}
	*nu = NodeUpdateRequest(b[0])
	return nil
}

func (nu NodeUpdateRequest) String() string {
	return fmt.Sprintf("#%d", nu)
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

func (nu *NodeUpdate) PackValue() ([]byte, error) {
	b := make([]byte, 18)
	b[0] = byte(nu.Id)
	b[1] = byte(nu.State)
	copy(b[2:10], nu.Udid[:])
	EncodeUint64(b[10:18], uint64(nu.LastSeen.UnixNano()))
	return b, nil
}

func (nu *NodeUpdate) UnpackValue(b []byte) error {
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

func (nl *NodeList) PackValue() ([]byte, error) {
	b := make([]byte, 0, len(nl.Nodes)*18)
	for _, nu := range nl.Nodes {
		sb, _ := nu.PackValue()
		b = append(b, sb...)
	}
	return b, nil
}

func (nl *NodeList) UnpackValue(b []byte) error {
	nl.Nodes = make([]*NodeUpdate, 0, 8)
	for {
		if len(b) == 0 {
			break
		}
		nu := &NodeUpdate{}
		if err := nu.UnpackValue(b); err != nil {
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

func (nf *NodeFirmware) PackValue() ([]byte, error) {
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

func (nf *NodeFirmware) UnpackValue(b []byte) error {
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

func (nfp *NodeFirmwareProgress) Failed() *NodeFirmwareProgress {
	return nfp.Update(ProgressFailed, 0)
}

func (nfp *NodeFirmwareProgress) Success() *NodeFirmwareProgress {
	return nfp.Update(ProgressSuccess, 0)
}

func (nfp *NodeFirmwareProgress) PackValue() ([]byte, error) {
	b := make([]byte, 6)
	b[0] = byte(nfp.Id)
	b[1] = byte(nfp.Progress)
	EncodeUint32(b[2:], nfp.BytesTransferred)
	return b, nil
}

func (nfp *NodeFirmwareProgress) UnpackValue(b []byte) error {
	if len(b) < 6 {
		return ErrorMissingData
	}
	nfp.Id = nocan.NodeId(b[0])
	nfp.Progress = ProgressReport(b[1])
	nfp.BytesTransferred = DecodeUint32(b[2:])
	return nil
}

//
//
//

type BusPower bool

func (bp BusPower) PackValue() ([]byte, error) {
	b := make([]byte, 1)
	if bp {
		b[0] = 1
	} else {
		b[0] = 0
	}
	return b, nil
}

func (bp *BusPower) UnpackValue(b []byte) error {
	if len(b) != 1 {
		return ErrorMissingData
	}
	if b[0] == 0 {
		*bp = false
	} else {
		*bp = true
	}
	return nil
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

func (nr NodeRebootRequest) PackValue() ([]byte, error) {
	b := make([]byte, 1)
	b[0] = byte(nr)
	return b, nil
}

func (nr *NodeRebootRequest) UnpackValue(b []byte) error {
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
	ClientAuthEvent                          = 2
	ClientSubscribeEvent                     = 3
	ServerAckEvent                           = 4
	ServerHelloEvent                         = 5
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
	"client-auth-event",
	"client-subscribe-event",
	"server-ack-event",
	"server-hello-event",
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

func registerEventHandlers(s *Server) {
	s.RegisterHandler(ClientHelloEvent, clientHelloHandler)
	s.RegisterHandler(ClientAuthEvent, clientAuthHandler)
	s.RegisterHandler(ClientSubscribeEvent, clientSubscribeHandler)
}

func init() {
	EventNameMap = make(map[string]EventId)
	for i, n := range EventNames {
		EventNameMap[n] = EventId(i)
	}
}
