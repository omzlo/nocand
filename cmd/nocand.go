package main

import (
	"flag"
	"fmt"
	"github.com/omzlo/nocand/clog"
	"github.com/omzlo/nocand/cmd/config"
	"github.com/omzlo/nocand/controllers"
	"github.com/omzlo/nocand/models/helpers"
	"os"
	"path"
	"runtime"
	"time"
)

var NOCAND_VERSION string = "Undefined"

var (
	optConfig string
)

func VersionFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.BoolVar(&config.Settings.CheckForUpdates, "check-for-updates", config.Settings.CheckForUpdates, "Check if a new version of nocanc is available")
	return fs
}

func HelpFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	return fs
}

func BaseFlagSet(cmd string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	fs.StringVar(&optConfig, "config", config.DefaultConfigLocation(), "Config file location.")
	fs.BoolVar(&config.Settings.DriverReset, "driver-reset", config.Settings.DriverReset, "Reset driver at startup (default: true).")
	fs.UintVar(&config.Settings.PowerMonitoringInterval, "power-monitoring-interval", config.Settings.PowerMonitoringInterval, "CANbus power monitoring interval in seconds (default: 10, disable with 0).")
	fs.UintVar(&config.Settings.SpiSpeed, "spi-speed", config.Settings.SpiSpeed, "SPI communication speed in bits per second (use with caution).")
	fs.UintVar(&config.Settings.LogLevel, "log-level", config.Settings.LogLevel, "Log level (0=all, 1=debug and more, 2=info and more, 3=warnings and errors, 4=errors only, 5=nothing)")
	fs.UintVar(&config.Settings.CurrentLimit, "current-limit", config.Settings.CurrentLimit, "Current limit level (default=0 -> don't change)")
	fs.StringVar(&config.Settings.LogFile, "log-file", config.DefaultLogLocation(), "Log file name, if empty no log file is created.")
	return fs
}

func PowerFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	return fs
}

var Commands = helpers.CommandFlagSetList{
	{"help", nil, HelpFlagSet, "help <command>", "Provide detailed help about a command"},
	{"power-on", poweron_cmd, BaseFlagSet, "power-on [options]", "Power on the NoCAN network and stopy"},
	{"power-off", poweroff_cmd, BaseFlagSet, "power-on [options]", "Power off the NoCAN network and stop"},
	{"server", server_cmd, BaseFlagSet, "server [options]", "Launch the NoCAN network manager and event server"},
	{"version", version_cmd, VersionFlagSet, "version", "display the version"},
}

func help_cmd(fs *flag.FlagSet) error {
	xargs := fs.Args()

	if len(xargs) == 0 {

		fmt.Printf("Usage:\r\n")
		fmt.Println(Commands.Usage())

	} else {
		if len(xargs) == 1 {
			c := Commands.Find(xargs[0])
			if c != nil {
				fmt.Printf("Usage:\r\n")
				fmt.Println(c.Usage())
			} else {
				fmt.Printf("Unknonwn command '%s'.\r\n", xargs[0])
				c := Commands.FuzzyMatch(xargs[0])
				if c != nil {
					fmt.Printf("Did you mean '%s'?\r\n", c.Command)
				}
			}
		} else {
			fmt.Printf("help does not accept more than one parameter.\r\n")
		}
	}
	if !config.Settings.Loaded {
		fmt.Printf("No configuration file was loaded: %s\r\n", config.Settings.LoadError)
	}
	return nil
}

func init_pimaster() error {
	var start_driver bool

	clog.SetLogLevel(clog.LogLevel(config.Settings.LogLevel))
	if config.Settings.LogFile != "" {
		clog.Info("Logs will be saved in %s", config.Settings.LogFile)
		clog.SetLogFile(config.Settings.LogFile)
	}

	if !config.Settings.Loaded {
		clog.Info("No configuration file was loaded: %s", config.Settings.LoadError)
	}
	clog.Info("nocand version %s", NOCAND_VERSION)

	if config.Settings.DriverReset {
		start_driver = controllers.BUS_RESET
	} else {
		start_driver = controllers.NO_BUS_RESET
	}

	if err := controllers.Bus.Initialize(start_driver, config.Settings.SpiSpeed); err != nil {
		return fmt.Errorf("Failed to connect to PiMaster.")
	}
	clog.Info("Successfully connected to PiMaster.")

	if config.Settings.CurrentLimit > 0 {
		controllers.Bus.SetCurrentLimit(uint16(config.Settings.CurrentLimit))
	}
	return nil
}

func server_cmd(fs *flag.FlagSet) error {
	if err := controllers.EventServer.ListenAndServe(config.Settings.Bind, config.Settings.AuthToken); err != nil {
		clog.Fatal("Failed to launch server: %s", err)
	}

	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(true)

	controllers.Bus.RunPowerMonitor(time.Duration(config.Settings.PowerMonitoringInterval) * time.Second)

	return controllers.Bus.Serve()
}

func poweron_cmd(fs *flag.FlagSet) error {
	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(true)

	return nil
}

func poweroff_cmd(fs *flag.FlagSet) error {
	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(false)

	return nil
}

func version_cmd(fs *flag.FlagSet) error {
	fmt.Printf("nocand version %s-%s-%s\r\n", NOCAND_VERSION, runtime.GOOS, runtime.GOARCH)
	if config.Settings.CheckForUpdates {
		fmt.Printf("\r\nChecking if a new version is available for download:\r\n")
		content, err := helpers.CheckForUpdates("http://omzlo.com/downloads/nocand.version")
		if err != nil {
			return err
		}
		if content[0] != NOCAND_VERSION {
			fmt.Printf(" - Version %s of nocand is available for download.\r\n", content[0])
			if len(content) > 1 {
				fmt.Printf(" - Release notes:\r\n%s\r\n", content[1])
			}
		} else {
			fmt.Printf(" - This version of nocand is up-to-date\r\n")
		}
	}
	fmt.Printf("\r\n")
	return nil
}

func main() {
	command, fs, err := Commands.Parse()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\r\n", err)
		fmt.Fprintf(os.Stderr, "type `%s help` for usage\r\n", path.Base(os.Args[0]))
		os.Exit(-2)
	}

	config.Load(optConfig)

	if command.Processor == nil {
		help_cmd(fs)
	} else {
		err = command.Processor(fs)

		if err != nil {
			fmt.Fprintf(os.Stderr, "# %s failed: %s\r\n", command.Command, err)
			os.Exit(-1)
		}
	}

	clog.Terminate()
}
