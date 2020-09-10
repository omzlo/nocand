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

func clientChannelUpdateRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	var cu *socket.ChannelUpdate
	var channel *models.Channel

	cur := e.Value.(*socket.ChannelUpdateRequest)
	if cur.Name == "" {
		channel = Channels.Find(cur.Id)
	} else {
		channel = Channels.Lookup(cur.Name)
	}

	if channel == nil {
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckNotFound)
	}

	cu = socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_UPDATED, channel.Value)
	return c.RespondToEvent(e, socket.ChannelUpdateEvent, cu)
}

func clientChannelUpdateHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	var channel *models.Channel

	cu := e.Value.(*socket.ChannelUpdate)

	if cu.Status == socket.CHANNEL_CREATED {
		_, err := Channels.Register(cu.Name)
		if err != nil {
			clog.Warning("Channel creation error for (%d, %s): %s", cu.Id, cu.Name, err)
			return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckGeneralFailure)
		}
	} else {

		if cu.Name == "" {
			channel = Channels.Find(cu.Id)
		} else {
			channel = Channels.Lookup(cu.Name)
		}

		if channel == nil {
			clog.Warning("Non-existing channel (%d, %s) in channel update event", cu.Id, cu.Name)
			return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckNotFound)
		}

		if cu.Status == socket.CHANNEL_UPDATED {
			channel.SetContent(cu.Value)
			Bus.Publish(0, channel.Id, cu.Value)
			clog.DebugXX("Broadcasting channel update on %s: %q", cu.Name, cu.Value)
			EventServer.Broadcast(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_UPDATED, cu.Value), c)
			return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
		}
		if cu.Status == socket.CHANNEL_DESTROYED {
			if !Channels.Unregister(channel) {
				clog.Warning("Could not unregister channel %s", cu.Name)
				return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckGeneralFailure)
			}
		}
	}
	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckUnknown)
}

func clientChannelListRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	cl := socket.NewChannelList()
	Channels.Each(func(c *models.Channel) {
		cl.Append(socket.NewChannelUpdate(c.Name, c.Id, socket.CHANNEL_UPDATED, c.Value))
	})

	return c.RespondToEvent(e, socket.ChannelListEvent, cl)
}

func clientNodeUpdateRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	var nu *socket.NodeUpdate

	nur := e.Value.(*socket.NodeUpdateRequest)

	node := Nodes.Find(nur.NodeId)

	if node == nil {
		nu = socket.NewNodeUpdate(nur.NodeId, models.NodeStateUnknown, models.NullUdid8, time.Unix(0, 0))
	} else {
		nu = socket.NewNodeUpdate(nur.NodeId, node.State, node.Udid, node.LastSeen)
	}
	return c.RespondToEvent(e, socket.NodeUpdateEvent, nu)
}

func clientNodeListRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	nl := socket.NewNodeList()
	Nodes.Each(func(n *models.Node) {
		nl.Append(socket.NewNodeUpdate(n.Id, n.State, n.Udid, n.LastSeen))
	})

	return c.RespondToEvent(e, socket.NodeListEvent, nl)
}

func clientFirmwareUploadHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	firmware := e.Value.(*socket.NodeFirmware)

	// sanity check
	for bid, bdata := range firmware.Code {
		if bdata.Offset < 0x2000 {
			clog.Warning("Node firmware block %d contains illegal offset 0x%x in bootloader reserved area.", bid, bdata.Offset)
			return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckBadRequest)
		}
	}

	node := Nodes.Find(firmware.Id)
	if node == nil {
		clog.Warning("Node firmware upload request failed: node %d does not exist", firmware.Id)
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckNotFound)
	}

	progress := socket.NewFirmwareProgress(firmware.Id)

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_UPLOAD_FLASH, progress, firmware)
	if err := Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil); err != nil {
		clog.Warning("Boot request for node %d firmware upload failed: %s", firmware.Id, err)
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckGeneralFailure)
	}
	progress.Update(0, 0)
	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
}

func clientFirmwareDownloadRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	firmware := e.Value.(*socket.NodeFirmware)

	node := Nodes.Find(firmware.Id)
	if node == nil {
		clog.Warning("Node firmware download request failed: node %d does not exist", firmware.Id)
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckNotFound)
	}

	progress := socket.NewFirmwareProgress(firmware.Id)

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_DOWNLOAD_FLASH, progress, firmware)
	if err := Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil); err != nil {
		clog.Warning("Boot request for node %d firmware download failed: %s", firmware.Id, err)
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckGeneralFailure)
	}
	progress.Update(0, 0)
	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
}

func clientNodeRebootRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	request := e.Value.(*socket.NodeRebootRequest)

	if !request.Force() {
		node := Nodes.Find(request.NodeId())
		if node == nil {
			return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckNotFound)
		}
	}
	Bus.SendSystemMessage(request.NodeId(), nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil)

	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
}

func clientBusPowerHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	power := e.Value.(*socket.BusPower)

	Bus.SetPower(power.PowerOn)

	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
}

func clientBusPowerUpdateRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	Bus.RequestPowerStatusUpdate()
	return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckSuccess)
}

func clientDeviceInformationRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	if DeviceInfo == nil {
		clog.Warning("Device information is not available.")
		return c.RespondToEvent(e, socket.ServerAckEvent, socket.ServerAckGeneralFailure)
	}
	return c.RespondToEvent(e, socket.DeviceInformationEvent, DeviceInfo)
}

func clientSystemPropertiesRequestHandler(c *socket.ClientDescriptor, e *socket.Event) error {
	return c.RespondToEvent(e, socket.SystemPropertiesEvent, SystemProperties)
}

func init() {
	EventServer = socket.NewServer()
	EventServer.RegisterHandler(socket.ChannelUpdateRequestEvent, clientChannelUpdateRequestHandler)
	EventServer.RegisterHandler(socket.ChannelUpdateEvent, clientChannelUpdateHandler)
	EventServer.RegisterHandler(socket.ChannelListRequestEvent, clientChannelListRequestHandler)
	EventServer.RegisterHandler(socket.NodeUpdateRequestEvent, clientNodeUpdateRequestHandler)
	EventServer.RegisterHandler(socket.NodeListRequestEvent, clientNodeListRequestHandler)
	EventServer.RegisterHandler(socket.NodeFirmwareUploadEvent, clientFirmwareUploadHandler)
	EventServer.RegisterHandler(socket.NodeFirmwareDownloadRequestEvent, clientFirmwareDownloadRequestHandler)
	EventServer.RegisterHandler(socket.NodeRebootRequestEvent, clientNodeRebootRequestHandler)
	EventServer.RegisterHandler(socket.BusPowerEvent, clientBusPowerHandler)
	EventServer.RegisterHandler(socket.BusPowerStatusUpdateRequestEvent, clientBusPowerUpdateRequestHandler)
	EventServer.RegisterHandler(socket.DeviceInformationRequestEvent, clientDeviceInformationRequestHandler)
	EventServer.RegisterHandler(socket.SystemPropertiesRequestEvent, clientSystemPropertiesRequestHandler)
}
