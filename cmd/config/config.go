package config

import (
	"github.com/BurntSushi/toml"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	Bind                    string
	AuthToken               string
	DriverReset             bool
	PowerMonitoringInterval uint
	SpiSpeed                uint
	LogLevel                uint
	CurrentLimit            uint
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
