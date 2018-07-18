package config

import (
	"errors"
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
	LogFile                 string `toml:"log-file"`
}

var Settings = Configuration{
	Bind:                    ":4242",
	AuthToken:               "password",
	DriverReset:             true,
	PowerMonitoringInterval: 10,
	SpiSpeed:                250000,
	LogLevel:                0,
	CurrentLimit:            0,
	LogFile:                 "nocand.log",
}

func DefaultConfigLocation() string {
	home := helpers.HomeDir()
	file, err := helpers.LocateFile(home, ".nocand", "config")
	if err != nil {
		return ""
	}
	return file
}

func Load(file string) error {

	if file == "" {
		return errors.New("No file")
	}

	if _, err := toml.DecodeFile(file, &Settings); err != nil {
		return err
	}

	return nil
}
