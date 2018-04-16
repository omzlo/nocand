package controllers

import (
	"github.com/omzlo/nocand/clog"
	"github.com/omzlo/nocand/models/rpi"
	"github.com/omzlo/nocand/socket"
	"time"
)

const (
	NO_BUS_RESET = false
	BUS_RESET    = true
)

func MilliAmpEstimation(c uint16) uint {
	var ma float64
	ma = 1000 * float64(c) / 4095 * 3.3 / 1120 * 2150
	return uint(ma)
}

func (nc *NocanNetworkController) RunPowerMonitor(interval time.Duration) {
	go func() {
		for {
			if rpi.DriverReady {
				ps, err := rpi.DriverUpdatePowerStatus()
				if err != nil {
					clog.Warning("Failed to read driver power status: %s", err)
				} else {
					clog.DebugX("Driver voltage=%.1f, current sense=%d (~ %d mA), reference voltage=%.2f, status(%x)=%s.", ps.Voltage, ps.CurrentSense, MilliAmpEstimation(ps.CurrentSense), ps.RefLevel, byte(ps.Status), ps.Status)
				}
				EventServer.Broadcast(socket.BusPowerStatusUpdateEvent, ps)
			}
			time.Sleep(interval)
		}
	}()
}

func (nc *NocanNetworkController) Initialize(with_reset bool, spi_speed int) error {
	return rpi.DriverInitialize(with_reset, spi_speed)
}

func (nc *NocanNetworkController) SetPower(power_on bool) {
	rpi.DriverSetPower(power_on)
	EventServer.Broadcast(socket.BusPowerEvent, power_on)
}

func (nci *NocanNetworkController) SetCurrentLimit(limit uint16) {
	rpi.DriverSetCurrentLimit(limit)
	clog.DebugX("Driver current limit set to %d (~ %d mA)", limit, MilliAmpEstimation(limit))
}
