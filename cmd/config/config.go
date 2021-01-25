package config

import (
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	Loaded                  bool              `toml:"-"`
	LoadError               error             `toml:"-"`
	Bind                    string            `toml:"bind"`
	AuthToken               string            `toml:"auth-token"`
	AuthTokenMinimumSize    int               `toml:"auth-token-minimum-size"`
	DriverReset             bool              `toml:"driver-reset"`
	PowerMonitoringInterval uint              `toml:"power-monitoring-interval"`
	PingInterval            uint              `toml:"ping-interval"`
	SpiSpeed                uint              `toml:"spi-speed"`
	LogLevel                clog.LogLevel     `toml:"log-level"`
	CurrentLimit            uint              `toml:"current-limit"`
	LogTerminal             string            `toml:"log-terminal"`
	LogFile                 *helpers.FilePath `toml:"log-file"`
	NodeCache               *helpers.FilePath `toml:"node-cache"`
	CheckForUpdates         bool              `toml:"check-for-updates"`
	TerminationResistor     bool              `toml:"termination-resistor"`
}

var Settings = Configuration{
	Loaded:                  true,
	LoadError:               nil,
	Bind:                    ":4242",
	AuthToken:               "password",
	AuthTokenMinimumSize:    24,
	DriverReset:             true,
	PowerMonitoringInterval: 10,
	PingInterval:            5000,
	SpiSpeed:                500000,
	LogLevel:                0,
	CurrentLimit:            0,
	LogTerminal:             "plain",
	LogFile:                 DefaultLogFile,
	NodeCache:               DefaultNodeCacheFile,
	CheckForUpdates:         true,
	TerminationResistor:     true,
}

var (
	DefaultNocancConfigFile *helpers.FilePath = helpers.HomeDir().Append(".nocanc.conf")
	DefaultConfigFile       *helpers.FilePath = helpers.HomeDir().Append(".nocand", "config")
	DefaultNodeCacheFile    *helpers.FilePath = helpers.HomeDir().Append(".nocand", "cache")
	DefaultLogFile          *helpers.FilePath = helpers.NewFilePath()
)
