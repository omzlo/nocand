package helpers

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func HomeDir() string {
	home := os.Getenv("HOME")
	if len(home) != 0 {
		return home
	}

	if runtime.GOOS == "windows" {
		drive := os.Getenv("HOMEDRIVE")
		path := os.Getenv("HOMEPATH")
		if len(drive) != 0 && len(path) != 0 {
			return drive + path
		}
		home = os.Getenv("USERPROFILE")
		if len(home) != 0 {
			return home
		}
	} else {
		whoami, err := user.Current()
		if err != nil {
			return ""
		}
		if len(whoami.HomeDir) != 0 {
			return whoami.HomeDir
		}
	}
	return ""
}

func LocateFile(elems ...string) (string, error) {
	if runtime.GOOS == "windows" {
		for i, elem := range elems {
			if strings.HasPrefix(elem, ".") {
				elems[i] = "_" + elem[1:]
			}
		}
	}
	vpath := filepath.Join(elems...)
	_, err := os.Stat(vpath)
	if err != nil {
		return vpath, err
	}

	return vpath, nil
}

/*
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
		return vpath, err
	}

	return vpath, nil
}
*/
