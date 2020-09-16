package main

import (
	"flag"
	"fmt"
	"github.com/omzlo/clog"
	"github.com/omzlo/nocand/cmd/config"
	"github.com/omzlo/nocand/controllers"
	"github.com/omzlo/nocand/models"
	"github.com/omzlo/nocand/models/helpers"
	"os"
	"path"
	"runtime"
	"time"
)

var NocandVersion string = "Undefined"

var (
	optConfig *helpers.FilePath = config.DefaultConfigFile
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
	fs.Var(optConfig, "config", fmt.Sprintf("Config file location, defaults to %s", config.DefaultConfigFile))
	fs.BoolVar(&config.Settings.DriverReset, "driver-reset", config.Settings.DriverReset, "Reset driver at startup (default: true).")
	fs.UintVar(&config.Settings.PowerMonitoringInterval, "power-monitoring-interval", config.Settings.PowerMonitoringInterval, "CANbus power monitoring interval in seconds (default: 10, disable with 0).")
	fs.UintVar(&config.Settings.SpiSpeed, "spi-speed", config.Settings.SpiSpeed, "SPI communication speed in bits per second (use with caution).")
	fs.Var(&config.Settings.LogLevel, "log-level", "Log verbosity level (DEBUGXX, DEBUGX, DEBUG, INFO, WARNING, ERROR or NONE)")
	fs.UintVar(&config.Settings.CurrentLimit, "current-limit", config.Settings.CurrentLimit, "Current limit level (default=0 -> don't change)")
	fs.Var(config.Settings.LogFile, "log-file", "Log file name, if empty no log file is created.")
	fs.StringVar(&config.Settings.LogTerminal, "log-terminal", config.Settings.LogTerminal, "Log to terminal (choices: 'plain', 'color' or 'none').")
	fs.BoolVar(&config.Settings.CanTermination, "can-termination", config.Settings.CanTermination, "Enable CAN bus 120 ohm termination resistor (default: true).")
	return fs
}

func ServerFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	fs.UintVar(&config.Settings.PingInterval, "ping-interval", config.Settings.PingInterval, "Node ping interval in milliseconds (experimental, defaults to 0 = disabled).")
	fs.Var(config.Settings.NodeCache, "node-cache", fmt.Sprintf("Node cache file name, defaults to '%s'. Set it to an empty string to disable node caching.", config.DefaultNodeCacheFile))
	return fs
}

func PowerFlagSet(cmd string) *flag.FlagSet {
	fs := BaseFlagSet(cmd)
	return fs
}

var Commands = helpers.CommandFlagSetList{
	{"help", nil, HelpFlagSet, "help <command>", "Provide detailed help about a command"},
	{"power-on", poweron_cmd, BaseFlagSet, "power-on [options]", "Power on the NoCAN network and start"},
	{"power-off", poweroff_cmd, BaseFlagSet, "power-off [options]", "Power off the NoCAN network and stop"},
	{"server", server_cmd, ServerFlagSet, "server [options]", "Launch the NoCAN network manager and event server"},
	{"version", version_cmd, VersionFlagSet, "version", "Display the version"},
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

func init_config() {
	clog.SetLogLevel(clog.LogLevel(config.Settings.LogLevel))

	if !config.Settings.LogFile.IsNull() {
		writer := clog.NewFileLogWriter(config.Settings.LogFile.String())
		if writer == nil {
			clog.Fatal("Could not create log file '%s'. Note: set log-file='' if you don't want to create a log file.", config.Settings.LogFile)
		}
		clog.AddWriter(writer)
		clog.Info("Logs will be saved in %s", config.Settings.LogFile)
	} else {
		clog.Debug("No logs will be saved to file (log-file configuration option is blank).")
	}

	clog.Info("nocand version %s", NocandVersion)
}

func init_pimaster() error {
	var start_driver bool

	if config.Settings.DriverReset {
		start_driver = controllers.BUS_RESET
	} else {
		start_driver = controllers.NO_BUS_RESET
	}

	if err := controllers.Bus.Initialize(start_driver, config.Settings.SpiSpeed); err != nil {
		return fmt.Errorf("Failed to connect to PiMaster: %s", err)
	}
	clog.Info("Successfully connected to PiMaster.")

	if config.Settings.CurrentLimit > 0 {
		controllers.Bus.SetCurrentLimit(uint16(config.Settings.CurrentLimit))
	}
	controllers.Bus.SetCanTermination(config.Settings.CanTermination)
	if config.Settings.CanTermination == false {
		clog.Info("Disabling on-board CAN bus termination")
	}
	return nil
}

func server_cmd(fs *flag.FlagSet) error {
	init_config()

	models.NodeCacheFile(config.Settings.NodeCache)

	b, _ := time.Now().UTC().MarshalText()
	controllers.SystemProperties.AddString("started_at", string(b))

	if err := controllers.EventServer.ListenAndServe(config.Settings.Bind, config.Settings.AuthToken); err != nil {
		clog.Fatal("Failed to launch server: %s", err)
	}

	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(true)

	controllers.Bus.RunPowerMonitor(time.Duration(config.Settings.PowerMonitoringInterval) * time.Second)

	controllers.Bus.RunPinger(time.Duration(config.Settings.PingInterval) * time.Millisecond)

	return controllers.Bus.Serve()
}

func poweron_cmd(fs *flag.FlagSet) error {
	init_config()

	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(true)

	return nil
}

func poweroff_cmd(fs *flag.FlagSet) error {
	init_config()

	if err := init_pimaster(); err != nil {
		return err
	}

	controllers.Bus.SetPower(false)

	return nil
}

func version_cmd(fs *flag.FlagSet) error {
	fmt.Printf("nocand version %s\r\n", controllers.SystemProperties.AsString("nocand_full_version"))
	if config.Settings.CheckForUpdates {
		fmt.Printf("\r\nChecking if a new version is available for download:\r\n")
		content, err := helpers.CheckForUpdates("https://www.omzlo.com/downloads/nocand.version")
		if err != nil {
			return err
		}
		if content[0] != controllers.SystemProperties.AsString("nocand_version") {
			var extension string

			fmt.Printf(" - Version %s of nocand is available for download.\r\n", content[0])
			if len(content) > 1 {
				fmt.Printf(" - Release notes:\r\n%s\r\n", content[1])
			}
			if runtime.GOOS == "windows" {
				extension = "zip"
			} else {
				extension = "tar.gz"
			}
			fmt.Printf(" - Download link: https://www.omzlo.com/downloads/nocand-%s-%s.%s\r\n", runtime.GOOS, runtime.GOARCH, extension)
		} else {
			fmt.Printf(" - This version of nocand is up-to-date\r\n")
		}
	}
	fmt.Printf("\r\n")
	return nil
}

func main() {
	loaded_a_config_file := ""

	controllers.SystemProperties.AddString("nocand_version", NocandVersion)
	controllers.SystemProperties.AddString("nocand_full_version", fmt.Sprintf("%s-%s-%s", NocandVersion, runtime.GOOS, runtime.GOARCH))

	conf_opt := helpers.CheckForConfigFlag()
	if conf_opt != nil {
		if err := helpers.LoadConfiguration(conf_opt, &config.Settings); err != nil {
			fmt.Fprintf(os.Stderr, "Cloud not load configuration file '%s': %s\r\n", conf_opt, err)
			os.Exit(-2)
		}
		loaded_a_config_file = conf_opt.String()
	} else {
		err := helpers.LoadConfiguration(config.DefaultConfigFile, &config.Settings)
		if err != nil && err != helpers.FileNotFound {
			fmt.Fprintf(os.Stderr, "Error in configuration file '%s': %s\r\n", config.DefaultConfigFile, err)
			os.Exit(-2)
		}
		if err != helpers.FileNotFound {
			loaded_a_config_file = config.DefaultConfigFile.String()
		}
	}

	command, fs, err := Commands.Parse()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse command line: %s\r\n", err)
		fmt.Fprintf(os.Stderr, "type `%s help` for usage\r\n", path.Base(os.Args[0]))
		os.Exit(-2)
	}

	switch config.Settings.LogTerminal {
	case "plain":
		clog.AddWriter(clog.PlainTerminal)
	case "color":
		clog.AddWriter(clog.ColorTerminal)
	case "none":
		// skip
	default:
		fmt.Fprintf(os.Stderr, "The log-terminal setting must be either 'plain', 'color' or 'none'.\r\n")
		os.Exit(-1)
	}

	if loaded_a_config_file != "" {
		clog.Info("Loaded configuration from '%s'", loaded_a_config_file)
	} else {
		clog.Warning("Configuration file '%s' does not exist and no configuration file was specified with '-config'. Using default configuration options.", config.DefaultConfigFile)
	}

	if command.Processor == nil {
		help_cmd(fs)
	} else {
		err = command.Processor(fs)

		if err != nil {
			fmt.Fprintf(os.Stderr, "# %s failed: %s\r\n", command.Command, err)
			os.Exit(-1)
		}
	}

	clog.Terminate(0)
}
