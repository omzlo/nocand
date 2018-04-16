package main

import (
	"flag"
	"github.com/omzlo/nocand/clog"
	"github.com/omzlo/nocand/controllers"
	"time"
)

var (
	optDriverReset             bool
	optPowerMonitoringInterval int
	optSpiSpeed                int
	optLogLevel                int
	optTest                    bool
	optPowerOn                 bool
	optCurrentLimit            uint
)

func init() {
	flag.BoolVar(&optPowerOn, "power-on", true, "Power bus after reset")
	flag.BoolVar(&optTest, "test", false, "Halt after reset, initialization, without lauching server")
	flag.BoolVar(&optDriverReset, "driver-reset", true, "Reset driver at startup (default: true).")
	flag.IntVar(&optPowerMonitoringInterval, "power-monitoring-interval", 10, "CANbus power monitoring interval in seconds (default: 10, disable with 0).")
	flag.IntVar(&optSpiSpeed, "spi-speed", 250000, "SPI communication speed in bits per second (use with caution).")
	flag.IntVar(&optLogLevel, "log-level", int(clog.DEBUGXX), "Log level (0=all, 1=debug and more, 2=info and more, 3=warnings and errors, 4=errors only, 5=nothing)")
	flag.UintVar(&optCurrentLimit, "current-limit", 0, "Current limit level (default=0 -> don't change)")
}

func main() {
	var start_driver bool

	flag.Parse()

	clog.SetLogFile("nocand.log")
	clog.SetLogLevel(clog.LogLevel(optLogLevel))

	//controllers.CreateUnpackerRegistry()

	if optDriverReset {
		start_driver = controllers.BUS_RESET
	} else {
		start_driver = controllers.NO_BUS_RESET
	}

	if err := controllers.Bus.Initialize(start_driver, optSpiSpeed); err != nil {
		panic(err)
	}

	controllers.Bus.SetPower(optPowerOn)

	if optCurrentLimit > 0 {
		controllers.Bus.SetCurrentLimit(uint16(optCurrentLimit))
	}

	controllers.Bus.RunPowerMonitor(time.Duration(optPowerMonitoringInterval) * time.Second)

	if !optTest {
		go controllers.Bus.Serve()

		clog.Error("Sever failed: %s", controllers.EventServer.ListenAndServe(":4242"))
	} else {
		clog.Info("Done.")
	}
}
