package controllers

import (
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/can"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/rpi"
	"github.com/omzlo/nocand/socket"
	"strconv"
	"time"
)

var Bus *NocanNetworkController = NewNocanNetworkController()
var Nodes *models.NodeCollection = models.NewNodeCollection()
var Channels *models.ChannelCollection = models.NewChannelCollection()

//
//
//
type NodeContext struct {
	pendingMessage           *nocan.Message
	pendingFirmwareOperation *NodeFirmwareOperation
	inputQueue               chan *nocan.Message
	terminateSignal          chan bool
	running                  bool
}

type NocanNetworkController struct {
	nodeContexts [128]NodeContext
}

func NewNocanNetworkController() *NocanNetworkController {
	return &NocanNetworkController{}
}

func (nc *NocanNetworkController) ReceiveMessage(nodeId nocan.NodeId) (*nocan.Message, error) {
	msg, more := <-nc.nodeContexts[nodeId].inputQueue
	if more {
		return msg, nil
	}
	return nil, fmt.Errorf("Receive message failed for node %d", nodeId)
}

const DEFAULT_EXPECT_TIMEOUT = 3 * time.Second

func (nc *NocanNetworkController) ExpectSystemMessage(nodeId nocan.NodeId, fn nocan.MessageType) (*nocan.Message, error) {
	ticker := time.NewTicker(DEFAULT_EXPECT_TIMEOUT)
	defer ticker.Stop()

	select {
	case msg := <-nc.nodeContexts[nodeId].inputQueue:
		if msg.IsSystemMessage() {
			rfn, _ := msg.SystemFunctionParam()
			if rfn == fn {
				return msg, nil
			}
			return nil, fmt.Errorf("Unexpected system message %s, while expecting %s.", rfn, fn)
		}
		return nil, fmt.Errorf("Unexpected publish message, while expecting system message %d.", fn)
	case <-ticker.C:
		return nil, fmt.Errorf("Timeout while waiting for system message %d", fn)
	}
}

func (nc *NocanNetworkController) SendMessage(msg *nocan.Message) error {
	var frame can.Frame
	var pos uint8

	clog.DebugX("** Sending %s **", msg)
	pos = 0
	for {
		frame.CanId = msg.CanId | can.CANID_MASK_EXTENDED
		if pos == 0 {
			frame.CanId |= nocan.NOCANID_MASK_FIRST
		}
		if msg.Dlc-pos > 8 {
			frame.Dlc = 8
		} else {
			frame.Dlc = msg.Dlc - pos
			frame.CanId |= nocan.NOCANID_MASK_LAST
		}
		copy(frame.Data[:], msg.Data[pos:pos+frame.Dlc])
		if err := rpi.DriverSendCanFrame(frame); err != nil {
			return err
		}
		pos += frame.Dlc
		if pos >= msg.Dlc {
			break
		}
	}
	return nil
}

func (nc *NocanNetworkController) SendSystemMessage(nodeId nocan.NodeId, fn nocan.MessageType, pm uint8, data []byte) error {
	msg := nocan.NewSystemMessage(nodeId, fn, pm, data)
	return nc.SendMessage(msg)
}

func (nc *NocanNetworkController) Publish(node nocan.NodeId, channel nocan.ChannelId, data []byte) error {
	msg := nocan.NewPublishMessage(node, channel, data)
	return nc.SendMessage(msg)
}

func (nc *NocanNetworkController) pinger(interval time.Duration) {
	var dequeue []*models.Node
	for {
		dequeue = nil

		Nodes.Each(func(node *models.Node) {
			if node.FirmwareVersion >= 3 {
				inactivity := time.Since(node.LastSeen)
				if inactivity > interval*2 {
					dequeue = append(dequeue, node)
				} else if inactivity > interval {
					nc.SendSystemMessage(node.Id, nocan.SYS_NODE_PING, 0, nil)
				}
			}
		})
		for _, node := range dequeue {
			clog.Info("Unregistering node %d due to unresponsiveness.", node.Id)
			node.State = models.NodeStateUnresponsive
			EventServer.Broadcast(socket.NodeUpdateEvent, socket.NewNodeUpdate(node.Id, node.State, node.Udid, node.LastSeen))
			Nodes.Unregister(node)
		}
		time.Sleep(interval)
	}
}

func (nc *NocanNetworkController) RunPinger(interval time.Duration) {
	if interval > 0 {
		clog.Debug("Node ping interval is set to %s", interval)
		go nc.pinger(interval)
	} else {
		clog.Debug("Node pinging is disabled")
	}
}

func (nc *NocanNetworkController) Serve() error {

	nc.nodeContexts[0].running = true
	nc.nodeContexts[0].inputQueue = make(chan *nocan.Message, 16)
	nc.nodeContexts[0].terminateSignal = make(chan bool)

	models.NodeCacheLoad()

	go nc.handleMasterNode()

	for {
		frame := <-rpi.CanRxChannel

		nodeId := (frame.CanId >> 21) & 0x7F

		if !frame.IsExtended() {
			clog.Warning("Expected extended CAN frame, discarding %s.", frame)
			continue
		}

		if frame.Dlc > 8 {
			clog.Error("Frame DLC is greater than 8, discarding %s.", frame)
			continue
		}

		if !nc.nodeContexts[nodeId].running { // sending message from an unregistered node?
			clog.Warning("Got a frame from unknown node %d, dicarding %s.", nodeId, frame)
			continue
		}

		if (frame.CanId & nocan.NOCANID_MASK_FIRST) != 0 {
			if nc.nodeContexts[nodeId].pendingMessage != nil {
				clog.Warning("Got frame with inconsistent first bit indicator, discarding.")
				continue
			}
			nc.nodeContexts[nodeId].pendingMessage = nocan.NewMessage(frame.CanId, frame.Data[:frame.Dlc])
		} else {
			if nc.nodeContexts[nodeId].pendingMessage == nil {
				clog.Warning("Got first frame with missing first bit indicator, discarding.")
				continue
			}
			nc.nodeContexts[nodeId].pendingMessage.AppendData(frame.Data[:frame.Dlc])
		}

		if (frame.CanId & nocan.NOCANID_MASK_LAST) != 0 {
			msg := nc.nodeContexts[nodeId].pendingMessage
			clog.Debug("** Received %s **", msg)
			nc.nodeContexts[nodeId].inputQueue <- msg
			nc.nodeContexts[nodeId].pendingMessage = nil // clear
		}
	}
}

func (nc *NocanNetworkController) handleMasterNode() {
MasterLoop:
	for {
		msg := <-nc.nodeContexts[0].inputQueue

		fn, param := msg.SystemFunctionParam()
		switch nocan.MessageType(fn) {
		case nocan.SYS_ADDRESS_REQUEST:
			udid := models.CreateUdid8(msg.Bytes())
			node, err := Nodes.Register(udid, param)
			if err != nil {
				clog.Error("NOCAN_SYS_ADDRESS_REQUEST: Failed to register device %s, %s", udid, err)
				continue MasterLoop
			} else {
				clog.Info("Device %s has been registered as node N%d (fw=%d)", udid, node.Id, param)
			}
			node.SetAttribute("ID", strconv.Itoa(int(node.Id)))

			if nc.nodeContexts[node.Id].running {
				// terminate existing goroutine
				nc.nodeContexts[node.Id].terminateSignal <- true
			}
			nc.nodeContexts[node.Id].running = true
			nc.nodeContexts[node.Id].inputQueue = make(chan *nocan.Message, 16)
			nc.nodeContexts[node.Id].terminateSignal = make(chan bool)
			go nc.handleBusNode(node)
			nc.SendSystemMessage(0, nocan.SYS_ADDRESS_CONFIGURE, uint8(node.Id), msg.Bytes())

		default:
			clog.Warning("Got unexpected message with null node id: %s", msg)
		}

	}
}

func (nc *NocanNetworkController) handleBusNode(node *models.Node) {

	inputQueue := nc.nodeContexts[node.Id].inputQueue
	terminateSignal := nc.nodeContexts[node.Id].terminateSignal

	for {
		select {
		case msg := <-inputQueue:
			if msg == nil {
				clog.Warning("Got NULL message for node N%d", node.Id)
			} else {
				nc.handleBusNodeMessage(node, msg)
			}
			node.Touch()

		case <-terminateSignal:
			close(inputQueue)
			// close input queue and terminate goroutine
			return
			// use a 'return': don't put a 'break' here, it will break from the select only.
		}
	}

}

func (nc *NocanNetworkController) handleBusNodeMessage(node *models.Node, msg *nocan.Message) {

	if msg.IsSystemMessage() {
		/* Case 1: system message */

		fn, _ := msg.SystemFunctionParam()
		switch nocan.MessageType(fn) {
		case nocan.SYS_ADDRESS_CONFIGURE_ACK:
			node.State = models.NodeStateConnected
			EventServer.Broadcast(socket.NodeUpdateEvent, socket.NewNodeUpdate(node.Id, node.State, node.Udid, node.LastSeen))

		case nocan.SYS_NODE_BOOT_ACK:
			node.State = models.NodeStateBootloader
			pendingFirmwareOperation := nc.nodeContexts[node.Id].pendingFirmwareOperation
			if pendingFirmwareOperation != nil {
				switch pendingFirmwareOperation.Operation {
				case NODE_OP_UPLOAD_FLASH:
					clog.Info("Initiating firmware upload for node %s", node)
					node.State = models.NodeStateProgramming
					if err := uploadFirmware(node, pendingFirmwareOperation); err != nil {
						clog.Warning("Firmware upload failed: %s", err)
					} else {
						clog.Info("Firmware upload succeeded for node %s", node)
					}
				case NODE_OP_DOWNLOAD_FLASH:
					clog.Info("Initializing firmware dowload for node %s", node)
					node.State = models.NodeStateProgramming
					if err := downloadFirmware(node, pendingFirmwareOperation); err != nil {
						clog.Warning("Firmware download failed: %s", err)
					} else {
						clog.Info("Firmware download succeeded for node %s", node)
					}
				default:
				}
			} else {
				// accelerate boot by sending bootloader exit request
				nc.SendSystemMessage(msg.NodeId(), nocan.SYS_BOOTLOADER_LEAVE, 0, nil)
			}
			nc.nodeContexts[node.Id].pendingFirmwareOperation = nil

		case nocan.SYS_BOOTLOADER_LEAVE_ACK:
			// Do nothing

		case nocan.SYS_NODE_PING_ACK:
			// Do nothing

		case nocan.SYS_CHANNEL_REGISTER:
			channel_name := node.ExpandAttributes(msg.DataToString())
			if channel_name != msg.DataToString() {
				clog.Debug("Interpolated channel name %s to %s", msg.DataToString(), channel_name)
			}
			channel, err := Channels.Register(channel_name)
			if err != nil {
				clog.Warning("SYS_CHANNEL_REGISTER: Failed to register channel %s for node %d, %s", channel_name, msg.NodeId(), err)
				nc.SendSystemMessage(msg.NodeId(), nocan.SYS_CHANNEL_REGISTER_ACK, 0xFF, nil)
			} else {
				clog.Info("Registered channel %s for node %d as %d", channel_name, msg.NodeId(), channel.Id)
				nc.SendSystemMessage(msg.NodeId(), nocan.SYS_CHANNEL_REGISTER_ACK, 0x00, channel.Id.ToBytes())
				EventServer.Broadcast(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_CREATED, nil))
			}

		case nocan.SYS_CHANNEL_LOOKUP:
			channel_name := node.ExpandAttributes(msg.DataToString())
			if channel_name != msg.DataToString() {
				clog.Debug("Interpolated channel name %s to %s", msg.DataToString(), channel_name)
			}
			channel := Channels.Lookup(channel_name)
			if channel != nil {
				clog.Info("Node %d succesfully found id=%d for channel %s", msg.NodeId(), channel.Id, channel_name)
				nc.SendSystemMessage(msg.NodeId(), nocan.SYS_CHANNEL_LOOKUP_ACK, 0x00, channel.Id.ToBytes())
			} else {
				clog.Warning("NOCAN_SYS_CHANNEL_LOOKUP: Node %d failed to find id for channel %s", msg.NodeId(), channel_name)
				nc.SendSystemMessage(msg.NodeId(), nocan.SYS_CHANNEL_LOOKUP_ACK, 0xFF, nil)
			}

		default:
			clog.Warning("Message of type %s from node %s was not processed", nocan.MessageType(fn), node)
		}
	} else {

		/* CAS 2b: Publish message from node X */

		channel := Channels.Find(msg.ChannelId())
		if channel != nil {
			clog.Info("Updated content of channel '%s' (id=%d) to %s", channel.Name, msg.ChannelId(), msg.Bytes())
			channel.SetContent(msg.Bytes())
			EventServer.Broadcast(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_UPDATED, msg.Bytes()))
		} else {
			clog.Warning("Could not unpdate non-existing channel %d for node %s", msg.ChannelId(), msg.NodeId())
		}
	}
}
