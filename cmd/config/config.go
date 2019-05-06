package config

import (
	"github.com/omzlo/nocand/models/helpers"
	"github.com/omzlo/clog"
)

type Configuration struct {
	Loaded                  bool              `toml:"-"`
	LoadError               error             `toml:"-"`
	Bind                    string            `toml:"bind"`
	AuthToken               string            `toml:"auth-token"`
	DriverReset             bool              `toml:"driver-reset"`
	PowerMonitoringInterval uint              `toml:"power-monitoring-interval"`
	PingInterval            uint              `toml:"ping-interval"`
	SpiSpeed                uint              `toml:"spi-speed"`
	LogLevel                clog.LogLevel     `toml:"log-level"`
	CurrentLimit            uint              `toml:"current-limit"`
	LogTerminal             string            `toml:"log-terminal"`
	LogFile                 *helpers.FilePath `toml:"log-file"`
	CheckForUpdates         bool              `toml:"check-for-updates"`
}

var Settings = Configuration{
	Loaded:                  true,
	LoadError:               nil,
	Bind:                    ":4242",
	AuthToken:               "password",
	DriverReset:             true,
	PowerMonitoringInterval: 10,
	PingInterval:            0,
	SpiSpeed:                250000,
	LogLevel:                0,
	CurrentLimit:            0,
	LogTerminal:             "plain",
	LogFile:                 DefaultLogFile,
	CheckForUpdates:         true,
}

var (
	DefaultConfigFile *helpers.FilePath = helpers.HomeDir().Append(".nocand", "config")
	DefaultLogFile    *helpers.FilePath = helpers.NewFilePath()
)


