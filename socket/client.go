package socket

import (
	"fmt"
	"github.com/omzlo/nocand/models/device"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/properties"
	"net"
)

/****************************************************************************/

// EventConn
//
//
type EventConn struct {
	Conn                   net.Conn
	Addr                   string
	AuthToken              string
	Connected              bool
	Subscriptions          *SubscriptionList
	onConnect              func(*EventConn) bool
	onError                func(*EventConn, error)
	onChannelUpdate        func(*EventConn, *ChannelUpdate) bool
	onNodeUpdate           func(*EventConn, *NodeUpdate) bool
	onChannelList          func(*EventConn, *ChannelList) bool
	onNodeList             func(*EventConn, *NodeList) bool
	onPowerStatusUpdate    func(*EventConn, *device.PowerStatus) bool
	onNodeFirmwareProgress func(*EventConn, *NodeFirmwareProgress) bool
	onDeviceInformation    func(*EventConn, *device.Information) bool
	onSystemProperties     func(*EventConn, *properties.Properties) bool
}

func NewEventConn(addr string, auth string) *EventConn {
	if addr == "" {
		addr = ":4242"
	}
	return &EventConn{Addr: addr, AuthToken: auth, Connected: false, Subscriptions: NewSubscriptionList()}
}

func (conn *EventConn) dial() error {
	tcp_conn, err := net.Dial("tcp", conn.Addr)
	if err != nil {
		return err
	}

	conn.Conn = tcp_conn

	if err := conn.Put(ClientHelloEvent, nil); err != nil {
		conn.Close()
		return err
	}
	eid, value, err := conn.Get()
	if eid != ServerHelloEvent {
		conn.Close()
		return fmt.Errorf("Expected ServerHelloEvent %d from server %s, got %d", ServerHelloEvent, conn.Addr, eid)
	}
	if len(value) != 4 || value[0] != 'E' || value[1] != 'M' || value[2] != 1 || value[3] != 0 {
		conn.Close()
		return fmt.Errorf("Unexpected response to ClientHelloEvent from server %s: %q", conn.Addr, value)
	}

	if err = conn.Put(ClientAuthEvent, []byte(conn.AuthToken)); err != nil {
		conn.Close()
		return err
	}
	if err = conn.GetAck(); err != nil {
		conn.Close()
		return err
	}
	conn.Connected = true
	return nil
}

func Dial(addr string, auth string) (*EventConn, error) {
	ec := NewEventConn(addr, auth)
	if err := ec.dial(); err != nil {
		return nil, err
	}
	return ec, nil
}

func (conn *EventConn) Subscribe(subs *SubscriptionList) error {
	if err := conn.Put(ClientSubscribeEvent, subs); err != nil {
		return err
	}
	conn.Subscriptions = subs
	return conn.GetAck()
}

func (conn *EventConn) GetAck() error {
	eid, val, err := conn.Get()
	if err != nil {
		return err
	}

	if eid != ServerAckEvent {
		return fmt.Errorf("Expected Ack event %d from server %s, got %d instead", ServerAckEvent, conn.Addr, eid)
	}

	var sack ServerAck

	if err = sack.UnpackValue(val); err != nil {
		return err
	}

	return sack.ToError()
}

func (conn *EventConn) Get() (EventId, []byte, error) {
	return DecodeFromStream(conn.Conn)
}

func (conn *EventConn) WaitFor(eid EventId) ([]byte, error) {
	for {
		reid, rdata, rerr := conn.Get()
		if rerr != nil {
			return nil, rerr
		}
		if reid == eid {
			return rdata, nil
		}
		if reid == ServerAckEvent {
			var ack ServerAck
			if err := ack.UnpackValue(rdata); err != nil {
				return nil, err
			}
			if ack != SERVER_ACK_SUCCESS {
				return nil, ack.ToError()
			}
		}
	}
}

func (conn *EventConn) Put(eid EventId, value interface{}) error {
	return EncodeToStream(conn.Conn, eid, value)
}

func (conn *EventConn) Close() error {
	conn.Connected = false
	return conn.Conn.Close()
}

/* * * * * * * * * * * *
 * Simplified interface
 *
 */

/* A. Event callbacks */

func (conn *EventConn) OnChannelUpdate(cb func(*EventConn, *ChannelUpdate) bool) {
	conn.Subscriptions.Add(ChannelUpdateEvent)
	conn.onChannelUpdate = cb
}

func (conn *EventConn) OnNodeUpdate(cb func(*EventConn, *NodeUpdate) bool) {
	conn.Subscriptions.Add(NodeUpdateEvent)
	conn.onNodeUpdate = cb
}

func (conn *EventConn) OnChannelList(cb func(*EventConn, *ChannelList) bool) {
	conn.Subscriptions.Add(ChannelListEvent)
	conn.onChannelList = cb
}

func (conn *EventConn) OnNodeList(cb func(*EventConn, *NodeList) bool) {
	conn.Subscriptions.Add(NodeListEvent)
	conn.onNodeList = cb
}

func (conn *EventConn) OnConnect(cb func(*EventConn) bool) {
	conn.onConnect = cb
}

func (conn *EventConn) OnPowerStatusUpdate(cb func(*EventConn, *device.PowerStatus) bool) {
	conn.Subscriptions.Add(BusPowerStatusUpdateEvent)
	conn.onPowerStatusUpdate = cb
}

func (conn *EventConn) OnError(cb func(*EventConn, error)) {
	conn.onError = cb
}

func (conn *EventConn) OnNodeFirmwareProgress(cb func(*EventConn, *NodeFirmwareProgress) bool) {
	conn.Subscriptions.Add(NodeFirmwareProgressEvent)
	conn.onNodeFirmwareProgress = cb
}

func (conn *EventConn) OnDeviceInformation(cb func(*EventConn, *device.Information) bool) {
	conn.Subscriptions.Add(DeviceInformationEvent)
	conn.onDeviceInformation = cb
}

func (conn *EventConn) OnSystemProperties(cb func(*EventConn, *properties.Properties) bool) {
	conn.Subscriptions.Add(SystemPropertiesEvent)
	conn.onSystemProperties = cb
}

/* B. Event request functions  */

func (conn *EventConn) RequestNodeList() error {
	if err := conn.Put(NodeListRequestEvent, nil); err != nil {
		return err
	}
	return nil
}

func (conn *EventConn) RequestNodeUpdate(nodeId nocan.NodeId) error {
	if err := conn.Put(NodeUpdateRequestEvent, NodeUpdateRequest(nodeId)); err != nil {
		return err
	}
	return nil
}

func (conn *EventConn) RequestChannelList() error {
	if err := conn.Put(ChannelListRequestEvent, nil); err != nil {
		return err
	}
	return nil
}

func (conn *EventConn) _requestChannelUpdate(channelId nocan.ChannelId, channelName string) error {
	if err := conn.Put(NodeUpdateRequestEvent, NewChannelUpdateRequest(channelName, channelId)); err != nil {
		return err
	}
	return nil

}

func (conn *EventConn) RequestChannelUpdate(channelId nocan.ChannelId) error {
	return conn._requestChannelUpdate(channelId, "")
}

func (conn *EventConn) RequestChannelUpdateByName(channelName string) error {
	return conn._requestChannelUpdate(nocan.UNDEFINED_CHANNEL, channelName)
}

func (conn *EventConn) _sendChannelUpdate(channelId nocan.ChannelId, channelName string, channelValue []byte) error {
	if err := conn.Put(ChannelUpdateEvent, NewChannelUpdate(channelName, channelId, CHANNEL_UPDATED, channelValue)); err != nil {
		return err
	}
	return nil
}

func (conn *EventConn) SendChannelUpdate(channelId nocan.ChannelId, channelValue []byte) error {
	return conn._sendChannelUpdate(channelId, "", channelValue)
}

func (conn *EventConn) SendChannelUpdateByName(channelName string, channelValue []byte) error {
	return conn._sendChannelUpdate(nocan.UNDEFINED_CHANNEL, channelName, channelValue)
}

func (conn *EventConn) RequestPowerStatusUpate() error {
	if err := conn.Put(BusPowerStatusUpdateRequestEvent, nil); err != nil {
		return err
	}
	return nil
}

/* C. Event processing loop */

func (conn *EventConn) processError(err error) error {
	if conn.Connected {
		conn.Close()
	}
	if conn.onError != nil {
		conn.onError(conn, err)
	}
	return err
}

func (conn *EventConn) ProcessEvents() error {

	if conn.Connected == false {
		if err := conn.dial(); err != nil {
			return conn.processError(err)
		}
	}

	if err := conn.Subscribe(conn.Subscriptions); err != nil {
		return conn.processError(err)
	}

	if conn.onConnect != nil {
		if conn.onConnect(conn) == false {
			return nil
		}
	}

	for {
		eid, data, err := conn.Get()
		if err != nil {
			return conn.processError(err)
		}

		switch eid {
		case ChannelUpdateEvent:
			if conn.onChannelUpdate != nil {
				cu := new(ChannelUpdate)
				if err = cu.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onChannelUpdate(conn, cu) == false {
					return nil
				}
			}
		case ChannelListEvent:
			if conn.onChannelList != nil {
				cl := new(ChannelList)
				if err = cl.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onChannelList(conn, cl) == false {
					return nil
				}
			}
		case NodeUpdateEvent:
			if conn.onNodeUpdate != nil {
				nu := new(NodeUpdate)
				if err = nu.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onNodeUpdate(conn, nu) == false {
					return nil
				}
			}
		case NodeListEvent:
			if conn.onNodeList != nil {
				nl := new(NodeList)
				if err = nl.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onNodeList(conn, nl) == false {
					return nil
				}
			}
		case BusPowerStatusUpdateEvent:
			if conn.onPowerStatusUpdate != nil {
				pu := new(device.PowerStatus)
				if err = pu.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onPowerStatusUpdate(conn, pu) == false {
					return nil
				}
			}
		case NodeFirmwareProgressEvent:
			if conn.onNodeFirmwareProgress != nil {
				np := new(NodeFirmwareProgress)
				if err = np.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onNodeFirmwareProgress(conn, np) == false {
					return nil
				}
			}
		case DeviceInformationEvent:
			if conn.onDeviceInformation != nil {
				di := new(device.Information)
				if err = di.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onDeviceInformation(conn, di) == false {
					return nil
				}
			}
		case SystemPropertiesEvent:
			if conn.onSystemProperties != nil {
				sp := properties.New()
				if err = sp.UnpackValue(data); err != nil {
					return conn.processError(err)
				}
				if conn.onSystemProperties(conn, sp) == false {
					return nil
				}
			}
		default:
			return fmt.Errorf("Unprocessed event %d with data %q", eid, data)
		}
	}
}
