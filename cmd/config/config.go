package config

import (
	"github.com/BurntSushi/toml"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	Bind                    string `toml:"bind"`
	AuthToken               string `toml:"auth-token"`
	DriverReset             bool   `toml:"driver-reset"`
	PowerMonitoringInterval uint   `toml:"power-monitoring-interval"`
	SpiSpeed                uint   `toml:"spi-speed"`
	LogLevel                uint   `toml:"log-level"`
	CurrentLimit            uint   `toml:"current-limit"`
}

var Settings = Configuration{
	Bind:                    ":4242:",
	AuthToken:               "password",
	DriverReset:             true,
	PowerMonitoringInterval: 10,
	SpiSpeed:                250000,
	LogLevel:                0,
	CurrentLimit:            0,
}

func Load() error {

	fn, err := helpers.LocateDotFile("nocand.conf")

	if err != nil {
		return err
	}

	if _, err := toml.DecodeFile(fn, &Settings); err != nil {
		return err
	}

	return nil
}
