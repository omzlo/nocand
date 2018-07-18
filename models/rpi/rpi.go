package rpi

/*
#cgo LDFLAGS: -lwiringPi
#include <wiringPiSPI.h>
#include "glue.h"
*/
import "C"
import "fmt"
import "encoding/hex"
import "github.com/omzlo/nocand/clog"
import "github.com/omzlo/nocand/models/can"
import "github.com/omzlo/nocand/models/device"
import "sync"
import "time"

const (
	SpiChannel C.int = 0
	SpiSpeed   C.int = 250000
)

const (
	WITHOUT_RESET = false
	WITH_RESET    = true
)

const (
	SPI_OP_NULL              = 0
	SPI_OP_RESET             = 1
	SPI_OP_DEVICE_INFO       = 2
	SPI_OP_POWER_LEVEL       = 3
	SPI_OP_SET_POWER         = 4
	SPI_OP_SET_CAN_RES       = 5
	SPI_OP_STATUS            = 6
	SPI_OP_STORE_DATA        = 7
	SPI_OP_SEND_REQ          = 8
	SPI_OP_FETCH_DATA        = 9
	SPI_OP_RECV_ACK          = 10
	SPI_OP_SET_CURRENT_LIMIT = 11
)

var spi_op_names = [...]string{
	"SPI_OP_NULL",
	"SPI_OP_RESET",
	"SPI_OP_DEVICE_INFO",
	"SPI_OP_POWER_LEVEL",
	"SPI_OP_SET_POWER",
	"SPI_OP_SET_CAN_RES",
	"SPI_OP_STATUS",
	"SPI_OP_STORE_DATA",
	"SPI_OP_SEND_REQ",
	"SPI_OP_FETCH_DATA",
	"SPI_OP_RECV_ACK",
	"SPI_OP_SET_CURRENT_LIMIT",
}

const (
	SPI_OK_BYTE   = 0x80
	SPI_MORE_BYTE = 0xA0
	SPI_ERR_BYTE  = 0xFF
)

var SPIMutex sync.Mutex
var CanTxChannel chan (can.Frame)
var CanRxChannel chan (can.Frame)
var DriverReady = false
var trCounter uint = 0

func SPITransfer(buf []byte) error {
	var block [128]C.uchar
	var counter uint

	SPIMutex.Lock()
	defer SPIMutex.Unlock()

	counter = trCounter
	trCounter++

	lbuf := len(buf)

	if lbuf > 128 {
		return fmt.Errorf("SPI.Transfer: data must be less than 128 bytes")
	}

	for i := 0; i < lbuf; i++ {
		block[i] = C.uchar(buf[i])
	}

	clog.DebugX("(%d) SPI SEND %d: %s (%s)", counter, lbuf, hex.EncodeToString(buf), spi_op_names[buf[0]])

	r := C.wiringPiSPIDataRW(SpiChannel, &block[0], C.int(len(buf)))

	if r < 0 {
		return fmt.Errorf("SPI.Transfer: transfer error")
	}

	for i := 0; i < lbuf; i++ {
		buf[i] = byte(block[i])
	}
	clog.DebugX("(%d) SPI RECV %d: %s", counter, lbuf, hex.EncodeToString(buf))
	return nil
}

func DriverReset() error {
	var buf [3]byte

	buf[0] = SPI_OP_RESET
	buf[1] = 2 // 1 for soft reset / 2 for hard reset

	return SPITransfer(buf[:])
}

func DriverReadDeviceInfo() (*device.Info, error) {
	var buf [19]byte
	buf[0] = SPI_OP_DEVICE_INFO

	if err := SPITransfer(buf[:]); err != nil {
		return nil, err
	}
	info := &device.Info{}
	copy(info.Signature[:], buf[1:5])
	info.VersionMajor = buf[5]
	info.VersionMinor = buf[6]
	copy(info.ChipId[:], buf[7:])
	return info, nil
}

/*
// DevicePowerStatus
//
//
type DevicePowerStatus struct {
	Status       DriverStatusByte
	Voltage      float32
	CurrentSense uint16
	RefLevel     float32
}

func (ps *DevicePowerStatus) String() string {
	return fmt.Sprintf("Driver voltage=%.1f, current sense=%d, reference voltage=%.2f, status(%x)=%s.", ps.Voltage, ps.CurrentSense, ps.RefLevel, byte(ps.Status), ps.Status)
}

func (ps *DevicePowerStatus) PackValue() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, ps)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
*/

func DriverUpdatePowerStatus() (*device.PowerStatus, error) {
	var buf [11]byte
	buf[0] = SPI_OP_POWER_LEVEL

	if !DriverReady {
		return nil, fmt.Errorf("Driver is not available")
	}

	if err := SPITransfer(buf[:]); err != nil {
		return nil, err
	}
	status := &device.PowerStatus{}

	// STATUS[0] -> BUF[1]
	// STATUS[1] -> BUF[2]
	// LEVELS[]  -> BUF[3] .. BUF[8]

	status.Status = device.StatusByte(buf[1])
	var val uint16 = (uint16(buf[4]) << 8) | uint16(buf[3])
	status.Voltage = 11 * 3.3 * float32(val) / float32(0xFFF)
	status.CurrentSense = (uint16(buf[6]) << 8) | uint16(buf[5])
	status.RefLevel = 3.3 * float32((uint16(buf[10])<<8)|uint16(buf[9])) / float32((uint16(buf[8])<<8)|uint16(buf[7]))
	return status, nil
}

func DriverSetPower(powered bool) error {
	var buf [2]byte
	buf[0] = SPI_OP_SET_POWER

	if powered {
		buf[1] = 1
	} else {
		buf[1] = 0
	}
	return SPITransfer(buf[:])
}

func DriverSetCurrentLimit(limit uint16) error {
	var buf [3]byte
	buf[0] = SPI_OP_SET_CURRENT_LIMIT
	buf[1] = byte(limit >> 8)
	buf[2] = byte(limit & 0xFF)

	return SPITransfer(buf[:])
}

func DriverSetCanResistor(set bool) error {
	var buf [2]byte
	buf[0] = SPI_OP_SET_CAN_RES

	if set {
		buf[1] = 1
	} else {
		buf[1] = 0
	}
	return SPITransfer(buf[:])
}

func DriverStatus() (device.StatusByte, error) {
	var buf [2]byte
	buf[0] = SPI_OP_STATUS

	if err := SPITransfer(buf[:]); err != nil {
		return 0, err
	}
	return device.StatusByte(buf[1]), nil
}

func DriverStoreDate(data []byte) error {
	var buf [15]byte
	buf[0] = SPI_OP_STORE_DATA
	buf[1] = 13

	if len(data) != 13 {
		return fmt.Errorf("Wrong data length, expected 13 bytes")
	}
	copy(buf[2:], data[:])
	return SPITransfer(buf[:])
}

func DriverSendReq() error {
	var buf [2]byte
	buf[0] = SPI_OP_SEND_REQ

	if err := SPITransfer(buf[:]); err != nil {
		return err
	}
	if buf[1] != 0x80 {
		return fmt.Errorf("Unexpected status code (0x%x) for SPI_OP_SEND_REQUEST: expected 0x80", buf[1])
	}
	return nil
}

func DriverRecvAck() error {
	var buf [2]byte
	buf[0] = SPI_OP_RECV_ACK

	if err := SPITransfer(buf[:]); err != nil {
		return err
	}
	if buf[1] != 0x80 {
		return fmt.Errorf("Unexpected status code (0x%x) for SPI_OP_RECV_ACK: expected 0x80", buf[1])
	}
	return nil

}

/***/

func DriverRecvCanFrame() (*can.Frame, error) {
	var buf [15]byte
	buf[0] = SPI_OP_FETCH_DATA

	if err := SPITransfer(buf[:]); err != nil {
		return nil, err
	}

	if buf[1] != 13 {
		return nil, fmt.Errorf("Expected 13 as first byte in can frame returned by SPI, got %d", buf[1])
	}

	frame, err := can.DecodeFrame(buf[2:])
	if err != nil {
		return nil, err
	}
	return frame, DriverRecvAck()
}

func driverSendCanFrame(frame *can.Frame) error {
	buf := make([]byte, 15, 15)

	buf[0] = SPI_OP_STORE_DATA
	buf[1] = 13

	if err := can.EncodeFrame(frame, buf[2:]); err != nil {
		return err
	}

	if err := SPITransfer(buf[:]); err != nil {
		return err
	}
	return DriverSendReq()
}

func DriverSendCanFrame(frame can.Frame) error {
	CanTxChannel <- frame
	return nil
}

func DriverCheckSignature() bool {
	info, err := DriverReadDeviceInfo()
	if err != nil {
		return false
	}
	clog.Info("Firmware version %d.%d, signature='%s', chip id=%s", info.VersionMajor, info.VersionMinor, string(info.Signature[:]), hex.EncodeToString(info.ChipId[:]))
	return (info.Signature[0] == 'C' && info.Signature[1] == 'A' && info.Signature[2] == 'N' && info.Signature[3] == '0')
}

func DriverInitialize(reset bool, speed uint) error {
	DriverReady = false

	C.setup_wiring_pi()

	r := C.wiringPiSPISetup(SpiChannel, C.int(speed))
	if r < 0 {
		return fmt.Errorf("Could not open SPI device")
	}
	clog.Info("Connected to driver using SPI interface at %d bps", speed)

	if reset {
		clog.Info("Reseting driver")
		if err := DriverReset(); err != nil {
			return err
		}
	}

	clog.DebugX("Waiting for TX line to be HIGH")
	for C.digitalReadTx() == 0 {
		time.Sleep(100 * time.Millisecond)
	}
	clog.DebugX("TX line is HIGH")

	if !DriverCheckSignature() {
		return fmt.Errorf("SPI driver signature check failed.")
	}
	clog.Info("Driver signature verified.")
	C.setup_interrupts()

	DriverReady = true

	return nil
}

//export CanRxInterrupt
func CanRxInterrupt() {

	for C.digitalReadRx() == 0 {
		frame, e := DriverRecvCanFrame()
		if e != nil {
			clog.Error(e.Error())
			break
		}
		clog.DebugXX("RECV FRAME %s", frame)
		CanRxChannel <- *frame
	}

	//clog.DebugXX("Got interrupt on RX pin")
}

/*
func RunPowerMonitor(interval time.Duration) {
	go func() {
		for {
			if DriverReady {
				ps, err := DriverUpdatePowerStatus()
				if err != nil {
					clog.Warning("Failed to read driver power status: %s", err)
				} else {
					clog.DebugX("Driver voltage=%.1f, current sense=%d, reference voltage=%.2f, status(%x)=%s.", ps.Voltage, ps.CurrentSense, ps.RefLevel, byte(ps.Status), ps.Status)
				}
			}
			time.Sleep(interval)
		}
	}()
}
*/

func init() {
	CanTxChannel = make(chan (can.Frame), 32)
	CanRxChannel = make(chan (can.Frame), 32)

	go func() {
		for {
			frame := <-CanTxChannel
			start := time.Now()
			for C.digitalReadTx() == 0 {
				now := time.Now()
				for C.digitalReadTx() == 0 && time.Since(now).Seconds() < 3 {
				}
				if C.digitalReadTx() == 0 {
					clog.Warning("Microcontroller transmission has been blocking for more than %d seconds on frame %s.", int(time.Since(start).Seconds()), frame)
				}
			}
			if err := driverSendCanFrame(&frame); err != nil {
				clog.Error("Failed to send CAN frame - %s", err)
			}
			clog.DebugXX("SEND FRAME %s", frame)

		}
	}()
	/*
		go func() {
			for {
				if DriverReady {
					for C.digitalReadRx() != 0 {
					}
					frame, e := DriverRecvCanFrame()
					if e != nil {
						clog.Error(e.Error())
						break
					}
					clog.Warning("RECV FRAME %s", frame)
					CanRxChannel <- frame
				}
			}
		}()
	*/
}
