package config

import (
	"github.com/BurntSushi/toml"
	"github.com/omzlo/nocand/models/helpers"
)

type Configuration struct {
	Bind      string
	AuthToken string
}

var Settings = Configuration{
	Bind:      ":4242:",
	AuthToken: "password",
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
