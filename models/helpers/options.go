package helpers

import (
	"errors"
	"flag"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path"
	"strings"
)

var FileNotFound error = errors.New("File not found")

type CommandFlagSet struct {
	Command   string
	Processor func(*flag.FlagSet) error
	Flags     func(string) *flag.FlagSet
	UsageText string
	HelpText  string
}

type CommandFlagSetList []*CommandFlagSet

func (cfsl CommandFlagSetList) Find(command string) *CommandFlagSet {
	for _, c := range cfsl {
		if c.Command == command {
			return c
		}
	}
	return nil
}

func (cfsl CommandFlagSetList) LongestPrefixMatch(command string) *CommandFlagSet {
	best_match_index := -1
	best_match_length := 0

	for i, c := range cfsl {
		if strings.HasPrefix(c.Command, command) {
			if len(command) > best_match_length {
				best_match_length = len(command)
				best_match_index = i
			}
		}
	}
	if best_match_length == 0 {
		return nil
	}
	return cfsl[best_match_index]
}

func similarity(a, b string) int {
	d := 0

	if len(a) > len(b) {
		c := a
		a = b
		b = c
	}

	for i := 0; i < len(a); i++ {
		p := strings.IndexByte(b, a[i])
		if p >= 0 {
			d += len(a) - p
		}
	}
	//d += (len(b) - len(a))
	return d
}

func (cfsl CommandFlagSetList) FuzzyMatch(command string) *CommandFlagSet {
	best_match_index := -1
	best_match_value := 1

	for i, c := range cfsl {
		d := similarity(c.Command, command)
		if d > best_match_value {
			best_match_value = d
			best_match_index = i
		}
	}
	if best_match_index < 0 {
		return nil
	}
	return cfsl[best_match_index]

}

func (cfs *CommandFlagSet) Usage() string {
	var s string
	flag_len := 2
	progname := path.Base(os.Args[0])

	cfs.Flags(cfs.Command).VisitAll(func(f *flag.Flag) {
		if len(f.Name) > flag_len {
			flag_len = len(f.Name)
		}
	})
	flen := fmt.Sprintf("%d", flag_len)

	s = cfs.HelpText + "\r\n\r\nUsage:\r\n"
	s += fmt.Sprintf("  %s %s\r\n\r\nFlags:\r\n", progname, cfs.UsageText)
	cfs.Flags(cfs.Command).VisitAll(func(f *flag.Flag) {
		s += fmt.Sprintf("  -%-"+flen+"s  %s\r\n", f.Name, f.Usage)
	})
	return s
}

func (cfsl CommandFlagSetList) Usage() string {
	var s string
	progname := path.Base(os.Args[0])
	cmd_len := 2
	for _, c := range cfsl {
		if cmd_len < len(c.Command) {
			cmd_len = len(c.Command)
		}
	}
	clen := fmt.Sprintf("%d", cmd_len)

	s = fmt.Sprintf("Usage:\r\n  %s <command> [flags]\r\n\r\nAvailable commands:\r\n", progname)

	for _, c := range cfsl {
		s += fmt.Sprintf("  %-"+clen+"s  %s\r\n", c.Command, c.HelpText)
	}

	return s
}

func (cfsl CommandFlagSetList) Parse() (*CommandFlagSet, *flag.FlagSet, error) {
	progname := path.Base(os.Args[0])

	if len(os.Args) < 2 {
		return nil, nil, fmt.Errorf("%s: Missing command", progname)
	}

	command := os.Args[1]

	c := cfsl.LongestPrefixMatch(command)
	if c == nil {
		emsg := fmt.Sprintf("Unknown command '%s'.", command)
		suggestion := cfsl.FuzzyMatch(os.Args[1])
		if suggestion != nil {
			emsg += fmt.Sprintf(" Did you mean '%s'?", suggestion.Command)
		}
		return nil, nil, fmt.Errorf("%s", emsg)
	}

	fs := c.Flags(c.Command)
	if err := fs.Parse(os.Args[2:]); err != nil {
		return nil, nil, err
	}

	return c, fs, nil
}

func CheckForConfigFlag() *FilePath {
	for k, opt := range os.Args {
		if opt[0] == '-' {
			opt = opt[1:]
			if opt[0] == '-' {
				opt = opt[1:]
			}
			if opt == "config" {
				if k < len(os.Args)+1 {
					return NewFilePath(os.Args[k+1])
				}
			}
			if strings.HasPrefix(opt, "config=") {
				return NewFilePath(strings.TrimPrefix(opt, "config="))
			}
		}
	}
	return nil
}

func LoadConfiguration(file *FilePath, settings interface{}) error {
	if !file.Exists() {
		return FileNotFound
	}

	md, err := toml.DecodeFile(file.String(), settings)
	if err != nil {
		return err
	}

	if len(md.Undecoded()) > 0 {
		r := "Unrecognized configuration keys: "
		for _, v := range md.Undecoded() {
			r += v.String()
		}
		return errors.New(r)
	}

	return nil
}
