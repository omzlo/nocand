package controllers

import (
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/models/properties"
	"github.com/omzlo/nocand/socket"
	"time"
)

var EventServer *socket.Server
var SystemProperties *properties.Properties = properties.New()

/*
func clientChannelCreateDestroyHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var cu socket.ChannelUpdate
	var err error

	err = cu.UnpackValue(value)
	if err == nil {
		if eid == socket.ChannelCreateEvent {
			_, err = Channels.Register(cu.Name)
		} else {
			channel := Channels.Lookup(cu.Name)
			if channel != nil {
				if !Channels.Unregister(channel) {
					err = fmt.Errorf("Could not unregister channel %s", cu.Name)
				}
			} else {
				err = fmt.Errorf("Channel %s does not exist", cu.Name)
			}
		}
	}
	return err
}
*/

func clientChannelUpdateRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var cur socket.ChannelUpdateRequest
	var cu *socket.ChannelUpdate
	var channel *models.Channel

	if err := cur.UnpackValue(value); err != nil {
		return err
	}

	if cur.Name == "" {
		channel = Channels.Find(cur.Id)
	} else {
		channel = Channels.Lookup(cur.Name)
	}

	if channel == nil {
		cu = socket.NewChannelUpdate(cur.Name, cur.Id, socket.CHANNEL_NOT_FOUND, nil)
	} else {
		cu = socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_UPDATED, channel.Value)
	}

	return c.Put(socket.ChannelUpdateEvent, cu)
}

func clientChannelUpdateHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var cu socket.ChannelUpdate
	var channel *models.Channel

	if err := cu.UnpackValue(value); err != nil {
		return err
	}

	if cu.Status == socket.CHANNEL_CREATED {
		_, err := Channels.Register(cu.Name)
		return err
	}

	if cu.Name == "" {
		channel = Channels.Find(cu.Id)
	} else {
		channel = Channels.Lookup(cu.Name)
	}

	if channel == nil {
		return fmt.Errorf("Non-existing channel (%d, %s) in channel update event", cu.Id, cu.Name)
	}

	if cu.Status == socket.CHANNEL_UPDATED {
		channel.SetContent(cu.Value)
		Bus.Publish(0, channel.Id, cu.Value)
		clog.DebugXX("Broadcasting %q", cu.Value)
		EventServer.Broadcast(socket.ChannelUpdateEvent, socket.NewChannelUpdate(channel.Name, channel.Id, socket.CHANNEL_UPDATED, cu.Value))
	}
	if cu.Status == socket.CHANNEL_DESTROYED {
		if !Channels.Unregister(channel) {
			return fmt.Errorf("Could not unregister channel %s", cu.Name)
		}
	}
	return nil
}

func clientChannelListRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	if len(value) != 0 {
		return fmt.Errorf("ChannelListRequestEvent has non-empty value (length=%d)", len(value))
	}

	cl := socket.NewChannelList()
	Channels.Each(func(c *models.Channel) {
		cl.Append(socket.NewChannelUpdate(c.Name, c.Id, socket.CHANNEL_UPDATED, c.Value))
	})

	return c.Put(socket.ChannelListEvent, cl)
}

func clientNodeUpdateRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var nur socket.NodeUpdateRequest
	var nu *socket.NodeUpdate

	if err := nur.UnpackValue(value); err != nil {
		return err
	}

	node := Nodes.Find(nocan.NodeId(nur))

	if node == nil {
		nu = socket.NewNodeUpdate(nocan.NodeId(nur), models.NodeStateUnknown, models.NullUdid8, time.Unix(0, 0))
	} else {
		nu = socket.NewNodeUpdate(nocan.NodeId(nur), node.State, node.Udid, node.LastSeen)
	}
	return c.Put(socket.NodeUpdateEvent, nu)
}

func clientNodeListRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	if len(value) != 0 {
		return fmt.Errorf("ChannelListRequestEvent has non-empty value (length=%d)", len(value))
	}

	nl := socket.NewNodeList()
	Nodes.Each(func(n *models.Node) {
		nl.Append(socket.NewNodeUpdate(n.Id, n.State, n.Udid, n.LastSeen))
	})

	return c.Put(socket.NodeListEvent, nl)
}

func clientFirmwareUploadHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	firmware := new(socket.NodeFirmware)

	if err := firmware.UnpackValue(value); err != nil {
		return err
	}

	progress := socket.NewFirmwareProgress(firmware.Id)

	// sanity check
	for bid, bdata := range firmware.Code {
		if bdata.Offset < 0x2000 {
			c.Put(socket.NodeFirmwareProgressEvent, progress.Failed())
			return fmt.Errorf("Node firmware block %d contains illegal offset 0x%x in bootloader reserved area.", bid, bdata.Offset)
		}
	}

	node := Nodes.Find(firmware.Id)
	if node == nil {
		c.Put(socket.NodeFirmwareProgressEvent, progress.Failed())
		return fmt.Errorf("Node firmware upload request failed: node %d does not exist", firmware.Id)
	}

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_UPLOAD_FLASH, progress, firmware)
	return Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil)
}

func clientFirmwareDownloadRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	firmware := new(socket.NodeFirmware)

	if err := firmware.UnpackValue(value); err != nil {
		return err
	}

	progress := socket.NewFirmwareProgress(firmware.Id)

	node := Nodes.Find(firmware.Id)
	if node == nil {
		c.Put(socket.NodeFirmwareProgressEvent, progress.Failed())
		return fmt.Errorf("Node firmware download request failed: node %d does not exist", firmware.Id)
	}

	Bus.nodeContexts[node.Id].pendingFirmwareOperation = NewNodeFirmwareOperation(c, NODE_OP_DOWNLOAD_FLASH, progress, firmware)
	return Bus.SendSystemMessage(node.Id, nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil)
}

func clientNodeRebootRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var request socket.NodeRebootRequest

	if err := request.UnpackValue(value); err != nil {
		return err
	}

  if !request.Force() {
    node := Nodes.Find(request.NodeId())
    if node == nil {
      return c.Put(socket.ServerAckEvent, socket.SERVER_NOT_FOUND)
    }
  }
  Bus.SendSystemMessage(request.NodeId(), nocan.SYS_NODE_BOOT_REQUEST, 0x01, nil)

	return c.Put(socket.ServerAckEvent, socket.SERVER_SUCCESS)
}

func clientBusPowerHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	var power socket.BusPower

	if err := power.UnpackValue(value); err != nil {
		return err
	}

	Bus.SetPower(bool(power))

	return nil
}

func clientBusPowerUpdateRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	Bus.RequestPowerStatusUpdate()
	return nil
}

func clientDeviceInformationRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	if DeviceInfo == nil {
		return fmt.Errorf("Device information is not available.")
	}
	return c.Put(socket.DeviceInformationEvent, DeviceInfo)
}

func clientSystemPropertiesRequestHandler(c *socket.Client, eid socket.EventId, value []byte) error {
	return c.Put(socket.SystemPropertiesEvent, SystemProperties)
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
