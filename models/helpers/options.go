package helpers

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
)

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
	best_match_value := 0

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
	progname := path.Base(os.Args[0])

	s = fmt.Sprintf("%s %s\r\n\t%s\r\n", progname, cfs.UsageText, cfs.HelpText)
	s += "This command takes the following options:\r\n"
	cfs.Flags(cfs.Command).VisitAll(func(f *flag.Flag) {
		sdefault := ""
		if len(f.DefValue) == 0 {
			sdefault = fmt.Sprintf(" (default %q)", f.DefValue)
		}
		s += fmt.Sprintf("\t-%s\r\n\t\t%s%s\r\n", f.Name, f.Usage, sdefault)
	})
	return s
}

func (cfsl CommandFlagSetList) Usage() string {
	var s string
	progname := path.Base(os.Args[0])

	for _, c := range cfsl {
		s += fmt.Sprintf("%s %s\r\n\t- %s\r\n", progname, c.UsageText, c.HelpText)
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
