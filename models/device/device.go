package device

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// Information
//
//
type Information struct {
	Type         [8]byte
	Signature    [4]byte
	VersionMajor byte
	VersionMinor byte
	ChipId       [12]byte
}

func (di *Information) String() string {
	return fmt.Sprintf("%s, firmware version=%d.%d, signature: '%s', chip_id: '%s'",
		string(di.Type[:]),
		di.VersionMajor,
		di.VersionMinor,
		string(di.Signature[:]),
		hex.EncodeToString(di.ChipId[:]))
}

func (di *Information) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"type":"%s","signature":"%s","version_major":"%d","version_minor":"%d","chip_id":"%s"}`,
		string(di.Type[:]),
		string(di.Signature[:]),
		di.VersionMajor,
		di.VersionMinor,
		hex.EncodeToString(di.ChipId[:]))), nil
}

// StatusByte
//
//
type StatusByte byte

const (
	STATUS_RX_PENDING = 0x01
	STATUS_TX_PENDING = 0x02
	STATUS_UNDEF_0    = 0x04
	STATUS_UNDEF_1    = 0x08
	STATUS_ERROR      = 0x10
	STATUS_FAULT      = 0x20
	STATUS_POWERED    = 0x40
	STATUS_CAN_RES    = 0x80
)

var StatusStrings = [...]string{
	"rx-pending",
	"tx-pending",
	"undefined_0",
	"undefined_1",
	"driver error",
	"electric fault",
	"powered",
	"resistor",
}

func (s StatusByte) Strings() []string {
	var i uint
	var r []string

	for i = 0; i < 8; i++ {
		if (s & (1 << i)) != 0 {
			r = append(r, StatusStrings[i])
		} else {
			if i == 6 {
				r = append(r, "unpowered")
			}
		}
	}
	return r
}

func (s StatusByte) String() string {
	var r string
	for _, s := range s.Strings() {
		r += "+" + s
	}
	return r
}

func (s StatusByte) MarshalJSON() ([]byte, error) {
	return []byte("\"" + strings.Join(s.Strings(), ", ") + "\""), nil
}

// PowerStatus
//
//
type PowerStatus struct {
	Status       StatusByte `json:"status"`
	Voltage      float32    `json:"voltage"`
	CurrentSense uint16     `json:"current_sense"`
	RefLevel     float32    `json:"reference_voltage"`
}
