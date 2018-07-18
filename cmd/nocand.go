package main

import (
	"flag"
	"github.com/omzlo/nocand/clog"
	"github.com/omzlo/nocand/cmd/config"
	"github.com/omzlo/nocand/controllers"
	"time"
)

var NOCAND_VERSION string = "Undefined"

var (
	//optDriverReset             bool
	//optPowerMonitoringInterval int
	//optSpiSpeed                int
	//optLogLevel                int
	optTest    bool
	optPowerOn bool
	optConfig  string
	//optCurrentLimit            uint
)

func init() {
	flag.BoolVar(&optPowerOn, "power-on", true, "Power bus after reset")
	flag.BoolVar(&optTest, "test", false, "Halt after reset, initialization, without lauching server")
	flag.StringVar(&optConfig, "config", config.DefaultConfigLocation(), "Config file location.")

	flag.BoolVar(&config.Settings.DriverReset, "driver-reset", config.Settings.DriverReset, "Reset driver at startup (default: true).")
	flag.UintVar(&config.Settings.PowerMonitoringInterval, "power-monitoring-interval", config.Settings.PowerMonitoringInterval, "CANbus power monitoring interval in seconds (default: 10, disable with 0).")
	flag.UintVar(&config.Settings.SpiSpeed, "spi-speed", config.Settings.SpiSpeed, "SPI communication speed in bits per second (use with caution).")
	flag.UintVar(&config.Settings.LogLevel, "log-level", config.Settings.LogLevel, "Log level (0=all, 1=debug and more, 2=info and more, 3=warnings and errors, 4=errors only, 5=nothing)")
	flag.UintVar(&config.Settings.CurrentLimit, "current-limit", config.Settings.CurrentLimit, "Current limit level (default=0 -> don't change)")
	flag.StringVar(&config.Settings.LogFile, "log-file", config.Settings.LogFile, "Log file, if empty no log file is created.")
}

func main() {
	var start_driver bool

	err_config := config.Load(optConfig)

	flag.Parse()

	if config.Settings.LogFile != "" {
		clog.SetLogFile(config.Settings.LogFile)
	}
	clog.SetLogLevel(clog.LogLevel(config.Settings.LogLevel))
	clog.Info("nocand version %s", NOCAND_VERSION)

	if err_config != nil {
		clog.Info("No configuration file was loaded (%s)", err_config)
	}

	if !optTest {
		if err := controllers.EventServer.ListenAndServe(config.Settings.Bind, config.Settings.AuthToken); err != nil {
			clog.Fatal("Failed to launch server: %s", err)
		}
	}

	if config.Settings.DriverReset {
		start_driver = controllers.BUS_RESET
	} else {
		start_driver = controllers.NO_BUS_RESET
	}

	if err := controllers.Bus.Initialize(start_driver, config.Settings.SpiSpeed); err != nil {
		clog.Fatal("Failed to connect to PiMaster.")
	}
	clog.Info("Successfully connected to PiMaster.")

	controllers.Bus.SetPower(optPowerOn)

	if config.Settings.CurrentLimit > 0 {
		controllers.Bus.SetCurrentLimit(uint16(config.Settings.CurrentLimit))
	}

	controllers.Bus.RunPowerMonitor(time.Duration(config.Settings.PowerMonitoringInterval) * time.Second)

	if !optTest {
		controllers.Bus.Serve()
	} else {
		clog.Terminate()
	}
}
