package controllers

import (
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/properties"
	"github.com/omzlo/nocand/socket"
	"time"
)

var EventServer *socket.Server
var SystemProperties *properties.Properties = properties.New()

func clientChannelUpdateRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	var channel *models.Channel

	cur := e.(*socket.ChannelUpdateRequestEvent)
	if cur.ChannelName == "" {
		channel = Channels.Find(cur.ChannelId)
	} else {
		channel = Channels.Lookup(cur.ChannelName)
	}

	if channel == nil {
		return c.SendAck(socket.ServerAckNotFound)
	}
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}

	return c.SendEvent(socket.NewChannelUpdateEvent(channel.Name, channel.Id, socket.CHANNEL_UPDATED, channel.Value))
}

func clientChannelUpdateHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	var channel *models.Channel

	cu := e.(*socket.ChannelUpdateEvent)

	if cu.Status == socket.CHANNEL_CREATED {
		_, err := Channels.Register(cu.ChannelName)
		if err != nil {
			clog.Warning("Channel creation error for (%d, %s): %s", cu.ChannelId, cu.ChannelName, err)
			return c.SendAck(socket.ServerAckGeneralFailure)
		}
	} else {

		if cu.ChannelName == "" {
			channel = Channels.Find(cu.ChannelId)
		} else {
			channel = Channels.Lookup(cu.ChannelName)
		}

		if channel == nil {
			clog.Warning("Non-existing channel (%d, %s) in channel update event", cu.ChannelId, cu.ChannelName)
			return c.SendAck(socket.ServerAckNotFound)
		}

		if cu.Status == socket.CHANNEL_UPDATED {
			channel.SetContent(cu.Value)
			Bus.Publish(0, channel.Id, cu.Value)
			clog.DebugXX("Broadcasting channel update on %s: %q", cu.ChannelName, cu.Value)
			EventServer.Broadcast(socket.NewChannelUpdateEvent(channel.Name, channel.Id, socket.CHANNEL_UPDATED, cu.Value), c)
			return c.SendAck(socket.ServerAckSuccess)
		}
		if cu.Status == socket.CHANNEL_DESTROYED {
			if !Channels.Unregister(channel) {
				clog.Warning("Could not unregister channel %s", cu.ChannelName)
				return c.SendAck(socket.ServerAckGeneralFailure)
			}
		}
	}
	return c.SendAck(socket.ServerAckGeneralFailure)
}

func clientChannelListRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	cl := socket.NewChannelListEvent()
	Channels.Each(func(c *models.Channel) {
		cl.Append(socket.NewChannelUpdateEvent(c.Name, c.Id, socket.CHANNEL_UPDATED, c.Value))
	})
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}
	return c.SendEvent(cl)
}

func clientNodeUpdateRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	var nu *socket.NodeUpdateEvent

	nur := e.(*socket.NodeUpdateRequestEvent)

	node := Nodes.Find(nur.NodeId)

	if node == nil {
		nu = socket.NewNodeUpdateEventWithParams(nur.NodeId, models.NodeStateUnknown, models.NullUdid8, time.Unix(0, 0))
	} else {
		nu = socket.NewNodeUpdateEventWithParams(nur.NodeId, node.State, node.Udid, node.LastSeen)
	}
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}
	return c.SendEvent(nu)
}

func clientNodeListRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	nl := socket.NewNodeListEvent()
	Nodes.Each(func(n *models.Node) {
		nl.Append(socket.NewNodeUpdateEventWithParams(n.Id, n.State, n.Udid, n.LastSeen))
	})
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}
	return c.SendEvent(nl)
}

func clientFirmwareUploadHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	firmware := e.(*socket.NodeFirmwareEvent)

	// sanity check
	for bid, bdata := range firmware.Code {
		if bdata.Offset < 0x2000 {
			clog.Warning("Node firmware block %d contains illegal offset 0x%x in bootloader reserved area.", bid, bdata.Offset)
			return c.SendAck(socket.ServerAckBadRequest)
		}
	}

	node := Nodes.Find(firmware.NodeId)
	if node == nil {
		clog.Warning("Node firmware upload request failed: node %d does not exist", firmware.NodeId)
		return c.SendAck(socket.ServerAckNotFound)
	}

	progress := socket.NewNodeFirmwareProgressEvent(firmware.NodeId)

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_UPLOAD_FLASH, progress, firmware)
	if err := Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil); err != nil {
		clog.Warning("Boot request for node %d firmware upload failed: %s", firmware.NodeId, err)
		return c.SendAck(socket.ServerAckGeneralFailure)
	}
	progress.Update(0, 0)
	return c.SendAck(socket.ServerAckSuccess)
}

func clientFirmwareDownloadRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	firmware := e.(*socket.NodeFirmwareEvent)

	node := Nodes.Find(firmware.NodeId)
	if node == nil {
		clog.Warning("Node firmware download request failed: node %d does not exist", firmware.NodeId)
		return c.SendAck(socket.ServerAckNotFound)
	}

	progress := socket.NewNodeFirmwareProgressEvent(firmware.NodeId)

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_DOWNLOAD_FLASH, progress, firmware)
	if err := Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil); err != nil {
		clog.Warning("Boot request for node %d firmware download failed: %s", firmware.NodeId, err)
		return c.SendAck(socket.ServerAckGeneralFailure)
	}
	progress.Update(0, 0)
	return c.SendAck(socket.ServerAckSuccess)
}

func clientNodeRebootRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	request := e.(*socket.NodeRebootRequestEvent)

	if !request.Forced() {
		node := Nodes.Find(request.NodeId())
		if node == nil {
			return c.SendAck(socket.ServerAckNotFound)
		}
	}
	Bus.SendSystemMessage(request.NodeId(), nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil)

	return c.SendAck(socket.ServerAckSuccess)
}

func clientBusPowerHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	power := e.(*socket.BusPowerEvent)

	Bus.SetPower(power.PowerOn)

	return c.SendAck(socket.ServerAckSuccess)
}

func clientBusPowerUpdateRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	Bus.RequestPowerStatusUpdate()
	return c.SendAck(socket.ServerAckSuccess)
}

func clientDeviceInformationRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	if DeviceInfo == nil {
		clog.Warning("Device information is not available.")
		return c.SendAck(socket.ServerAckGeneralFailure)
	}
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}
	return c.SendEvent(socket.NewDeviceInformationEvent(DeviceInfo))
}

func clientSystemPropertiesRequestHandler(c *socket.ClientDescriptor, e socket.Eventer) error {
	if err := c.SendAck(socket.ServerAckSuccess); err != nil {
		return err
	}
	return c.SendEvent(socket.NewSystemPropertiesEvent(SystemProperties))
}

func init() {
	EventServer = socket.NewServer()
	EventServer.RegisterHandler(socket.ChannelUpdateRequestEventId, clientChannelUpdateRequestHandler)
	EventServer.RegisterHandler(socket.ChannelUpdateEventId, clientChannelUpdateHandler)
	EventServer.RegisterHandler(socket.ChannelListRequestEventId, clientChannelListRequestHandler)
	EventServer.RegisterHandler(socket.NodeUpdateRequestEventId, clientNodeUpdateRequestHandler)
	EventServer.RegisterHandler(socket.NodeListRequestEventId, clientNodeListRequestHandler)
	EventServer.RegisterHandler(socket.NodeFirmwareEventId, clientFirmwareUploadHandler)
	EventServer.RegisterHandler(socket.NodeFirmwareDownloadRequestEventId, clientFirmwareDownloadRequestHandler)
	EventServer.RegisterHandler(socket.NodeRebootRequestEventId, clientNodeRebootRequestHandler)
	EventServer.RegisterHandler(socket.BusPowerEventId, clientBusPowerHandler)
	EventServer.RegisterHandler(socket.BusPowerStatusUpdateRequestEventId, clientBusPowerUpdateRequestHandler)
	EventServer.RegisterHandler(socket.DeviceInformationRequestEventId, clientDeviceInformationRequestHandler)
	EventServer.RegisterHandler(socket.SystemPropertiesRequestEventId, clientSystemPropertiesRequestHandler)
}
