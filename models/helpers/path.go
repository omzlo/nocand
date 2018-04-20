package helpers

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"runtime"
)

func HomeDir() (string, error) {
	home := os.Getenv("HOME")
	if len(home) != 0 {
		return home, nil
	}

	if runtime.GOOS == "windows" {
		drive := os.Getenv("HOMEDRIVE")
		path := os.Getenv("HOMEPATH")
		if len(drive) != 0 && len(path) != 0 {
			return drive + path, nil
		}
		home = os.Getenv("USERPROFILE")
		if len(home) != 0 {
			return home, nil
		}
	} else {
		whoami, err := user.Current()
		if err != nil {
			return "", err
		}
		if len(whoami.HomeDir) != 0 {
			return whoami.HomeDir, nil
		}
	}
	return "", fmt.Errorf("Homedir was not found")
}

func LocateDotFile(fname string) (string, error) {
	var vpath string

	homedir, err := HomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		vpath = path.Clean(homedir + "\\_" + fname)
	} else {
		vpath = path.Clean(homedir + "/." + fname)
	}

	_, err = os.Stat(vpath)
	if err != nil {
		return "", err
	}
	return vpath, nil
}
