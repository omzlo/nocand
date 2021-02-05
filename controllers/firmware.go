package controllers

import (
	"fmt"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/nocan"
	"github.com/omzlo/nocand/socket"
	"hash/crc32"
)

const (
	NODE_OP_NONE = iota
	NODE_OP_UPLOAD_FLASH
	NODE_OP_DOWNLOAD_FLASH
)

type NodeFirmwareOperation struct {
	Client    *socket.ClientDescriptor
	Operation int // NODE_OP_...
	Progress  *socket.NodeFirmwareProgressEvent
	Firmware  *socket.NodeFirmwareEvent
}

func NewNodeFirmwareOperation(client *socket.ClientDescriptor, op int, progress *socket.NodeFirmwareProgressEvent, firmware *socket.NodeFirmwareEvent) *NodeFirmwareOperation {
	return &NodeFirmwareOperation{Client: client, Operation: op, Progress: progress, Firmware: firmware}
}

/*
 * Currently there is only one model for nodes, so we can use consts.
 * In the future we will need a 'per-device' approach.
 */
const (
	FLASH_ORIGIN     uint32 = 0
	FLASH_LENGTH     uint32 = 0x40000
	FLASH_APP_ORIGIN uint32 = FLASH_ORIGIN + 0x2000 // 8K bootloader
	FLASH_APP_LENGTH uint32 = FLASH_LENGTH - 0x2000
	FLASH_PAGE_SIZE  uint32 = 64
)

var FLASH_DEVICE_SIGNATURE = [4]byte{0x10, 0x01, 0x00, 0x05}

func uint32ToBytes(u uint32, d []byte) []byte {
	d[0] = byte(u >> 24)
	d[1] = byte(u >> 16)
	d[2] = byte(u >> 8)
	d[3] = byte(u)
	return d[:4]
}

func bytesToUint32(d []byte) uint32 {
	return (uint32(d[0]) << 24) | (uint32(d[1]) << 16) | (uint32(d[0]) << 8) | (uint32(d[0]))
}

func checkDeviceSignature(node *models.Node, op *NodeFirmwareOperation) error {
	Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_GET_SIGNATURE, 0, nil)
	response, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_GET_SIGNATURE_ACK)
	if err != nil {
		op.Client.SendEvent(op.Progress.MarkAsFailed())
		return err
	}
	if response.Dlc < 4 || response.Dlc > 8 {
		op.Client.SendEvent(op.Progress.MarkAsFailed())
		return fmt.Errorf("Unexpected length (%d bytes).", response.Dlc)
	}
	if response.Data[0] != FLASH_DEVICE_SIGNATURE[0] ||
		response.Data[1] != FLASH_DEVICE_SIGNATURE[1] ||
		response.Data[3] != FLASH_DEVICE_SIGNATURE[3] {
		op.Client.SendEvent(op.Progress.MarkAsFailed())
		return fmt.Errorf("Unexpected value: %v", response.Bytes())
	}
	return nil
}

func uploadFirmware(node *models.Node, op *NodeFirmwareOperation) error {
	var address uint32
	var crc uint32
	var data [64]byte
	var total_uploaded uint32 = 0

	err := checkDeviceSignature(node, op)
	if err != nil {
		return fmt.Errorf("Failed to get device signature for node %s, %s", node, err)
	}

	uint32ToBytes(FLASH_APP_ORIGIN, data[:])
	Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_SET_ADDRESS, 'F', data[:4])
	if _, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_SET_ADDRESS_ACK); err != nil {
		op.Client.SendEvent(op.Progress.MarkAsFailed())
		return fmt.Errorf("SYS_BOOTLOADER_SET_ADDRESS failed for node %s, prior to erase operation, %s", node, err)
	}

	Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_ERASE, 0, nil)
	if _, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_ERASE_ACK); err != nil {
		op.Client.SendEvent(op.Progress.MarkAsFailed())
		return fmt.Errorf("SYS_BOOTLOADER_ERASE failed for node %s, %s", node, err)
	}
	// TODO: check return code in ACK
	for _, block := range op.Firmware.Code {
		blocksize := uint32(len(block.Data))

		for page_offset := uint32(0); page_offset < blocksize; page_offset += FLASH_PAGE_SIZE {
			base_address := block.Offset + page_offset
			uint32ToBytes(base_address, data[:])
			Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_SET_ADDRESS, 'F', data[:4])
			if _, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_SET_ADDRESS_ACK); err != nil {
				op.Client.SendEvent(op.Progress.MarkAsFailed())
				return fmt.Errorf("SYS_BOOTLOADER_SET_ADDRESS failed for node %s at address=0x%x, %s", node, address, err)
			}

			crc = 0
			// This loop will only run once and could be simplifed because FLASH_PAGE_SIZE==64 for the SAMD21G18
			// But in the future we might have more complex cases.
			for page_pos := uint32(0); (page_pos < FLASH_PAGE_SIZE) && (page_offset+page_pos < blocksize); page_pos += 64 {
				rlen := copy(data[:], block.Data[page_offset+page_pos:])
				crc = crc32.Update(crc, crc32.IEEETable, data[:rlen])
				Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_WRITE, 0, data[:rlen])
				if _, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_WRITE_ACK); err != nil {
					op.Client.SendEvent(op.Progress.MarkAsFailed())
					return fmt.Errorf("SYS_BOOTLOADER_WRITE failed for node %d at address=0x%x, %s", node.Id, address, err)
				}
				total_uploaded += uint32(rlen)
			}
			uint32ToBytes(crc, data[:])
			Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_WRITE, 1, data[:4])

			response, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_WRITE_ACK)
			if err != nil {
				op.Client.SendEvent(op.Progress.MarkAsFailed())
				return fmt.Errorf("Final SYS_BOOTLOADER_WRITE failed for node %d at address=0x%x, %s", node.Id, address, err)
			}
			if response.SystemParam() == 0xFF {
				crc_r := bytesToUint32(response.Bytes())
				op.Client.SendEvent(op.Progress.MarkAsFailed())
				return fmt.Errorf("SYS_BOOTLOADER_WRITE failed for node %d at address=0x%x, CRC32 mismatch, expected=%x go %x", node.Id, address, crc, crc_r)
			}

			// TODO: check return code in ACK
			if err := op.Client.SendEvent(op.Progress.Update(socket.ProgressReport((page_offset*100)/blocksize), total_uploaded)); err != nil {
				return err
			}
		}
	}
	Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_LEAVE, 0, nil)

	op.Client.SendEvent(op.Progress.Update(socket.ProgressReport(100), total_uploaded))
	return op.Client.SendEvent(op.Progress.MarkAsSuccess())
}

func downloadFirmware(node *models.Node, op *NodeFirmwareOperation) error {
	var address, memlength uint32
	var i uint32
	var data [8]byte

	if op.Firmware.Limit > FLASH_APP_LENGTH || op.Firmware.Limit == 0 {
		memlength = FLASH_APP_LENGTH
	} else {
		memlength = op.Firmware.Limit
	}
	block := make([]byte, 0, memlength)

	/*
		err := checkDeviceSignature(node, op)
		if err != nil {
			return fmt.Errorf("Failed to get device signature for node %s, %s", node, err)
		}
	*/

	for i = 0; i < (memlength+FLASH_PAGE_SIZE-1)/FLASH_PAGE_SIZE; i++ {
		address = FLASH_APP_ORIGIN + i*FLASH_PAGE_SIZE
		uint32ToBytes(address, data[:])
		Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_SET_ADDRESS, 'F', data[:4])
		if _, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_SET_ADDRESS_ACK); err != nil {
			op.Client.SendEvent(op.Progress.MarkAsFailed())
			return fmt.Errorf("NOCAN_SYS_BOOTLOADER_SET_ADDRESS failed for node %d at address=0x%x, %s", node.Id, address, err)
		}

		for pos := uint32(0); pos < FLASH_PAGE_SIZE; pos += 64 {
			Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_READ, 64, nil)
			response, err := Bus.ExpectSystemMessage(node, nocan.SYS_BOOTLOADER_READ_ACK)
			if err != nil {
				op.Client.SendEvent(op.Progress.MarkAsFailed())
				return fmt.Errorf("NOCAN_SYS_BOOTLOADER_READ failed for node %d at address=0x%x, %s", node.Id, address, err)
			}

			block = append(block, response.Bytes()...)
			address += 64
		}
		if err := op.Client.SendEvent(op.Progress.Update(socket.ProgressReport((address-FLASH_APP_ORIGIN)*100/memlength), address-FLASH_APP_ORIGIN)); err != nil {
			return err
		}
	}
	op.Client.SendEvent(op.Progress.Update(socket.ProgressReport(100), memlength))
	op.Client.SendEvent(op.Progress.MarkAsSuccess())

	op.Firmware.AppendBlock(FLASH_APP_ORIGIN, block)

	Bus.SendSystemMessage(node.Id, nocan.SYS_BOOTLOADER_LEAVE, 0, nil)
	return op.Client.SendEvent(op.Firmware)
}
