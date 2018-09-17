package config

import (
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	Loaded                  bool   `toml:"-"`
	LoadError               error  `toml:"-"`
	Bind                    string `toml:"bind"`
	AuthToken               string `toml:"auth-token"`
	DriverReset             bool   `toml:"driver-reset"`
	PowerMonitoringInterval uint   `toml:"power-monitoring-interval"`
	SpiSpeed                uint   `toml:"spi-speed"`
	LogLevel                uint   `toml:"log-level"`
	CurrentLimit            uint   `toml:"current-limit"`
	LogFile                 string `toml:"log-file"`
	CheckForUpdates         bool   `toml:"check-for-updates"`
}

var Settings = Configuration{
	Loaded:                  true,
	LoadError:               nil,
	Bind:                    ":4242",
	AuthToken:               "password",
	DriverReset:             true,
	PowerMonitoringInterval: 10,
	SpiSpeed:                250000,
	LogLevel:                0,
	CurrentLimit:            0,
	LogFile:                 "nocand.log",
	CheckForUpdates:         true,
}

func DefaultConfigLocation() string {
	home := helpers.HomeDir()
	file, err := helpers.LocateFile(home, ".nocand", "config")
	if err != nil {
		return ""
	}
	return file
}

func DefaultLogLocation() string {
	home := helpers.HomeDir()
	file, err := helpers.LocateFile(home, ".nocand", "log")
	if err != nil {
		return ""
	}
	return file
}

func Load(file string) error {
	Settings.Loaded = false
	Settings.LoadError = nil

	if file == "" {
		Settings.LoadError = errors.New("No file")
		return Settings.LoadError
	}

	if _, err := toml.DecodeFile(file, &Settings); err != nil {
		Settings.LoadError = err
		return err
	}

	Settings.Loaded = true
	return nil
}
