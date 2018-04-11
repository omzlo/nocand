package controllers

import (
	"pannetrat.com/nocand/clog"
	"pannetrat.com/nocand/models/rpi"
	"pannetrat.com/nocand/socket"
	"time"
)

const (
	NO_BUS_RESET = false
	BUS_RESET    = true
)

func (nc *NocanNetworkController) RunPowerMonitor(interval time.Duration) {
	go func() {
		for {
			if rpi.DriverReady {
				ps, err := rpi.DriverUpdatePowerStatus()
				if err != nil {
					clog.Warning("Failed to read driver power status: %s", err)
				} else {
					clog.DebugX("Driver voltage=%.1f, current sense=%d, reference voltage=%.2f, status(%x)=%s.", ps.Voltage, ps.CurrentSense, ps.RefLevel, byte(ps.Status), ps.Status)
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
