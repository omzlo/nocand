package helpers

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

type FilePath struct {
	path string
}

func NewFilePath(elems ...string) *FilePath {
	fp := new(FilePath)
	fp.Append(elems...)
	return fp
}

func (file *FilePath) Append(elems ...string) *FilePath {
	if file == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		for i, elem := range elems {
			if strings.HasPrefix(elem, ".") {
				elems[i] = "_" + elem[1:]
			}
		}
	}
	suffix := filepath.Join(elems...)
	if file.path != "" {
		file.path = filepath.Join(file.path, suffix)
	} else {
		file.path = suffix
	}
	return file
}

func (file *FilePath) UnmarshalText(text []byte) error {
	return file.Set(string(text))
}

func (file *FilePath) Set(s string) error {
	file.path = s
	return nil
}

func (file *FilePath) String() string {
	return file.path
}

func (file *FilePath) Exists() bool {
	if _, err := os.Stat(file.path); os.IsNotExist(err) {
		return false
	}
	return true
}

func (file *FilePath) IsNull() bool {
	return file.path == ""
}

func HomeDir() *FilePath {
	home := os.Getenv("HOME")
	if len(home) != 0 {
		return NewFilePath(home)
	}

	if runtime.GOOS == "windows" {
		drive := os.Getenv("HOMEDRIVE")
		path := os.Getenv("HOMEPATH")
		if len(drive) != 0 && len(path) != 0 {
			return NewFilePath(drive + path)
		}
		home = os.Getenv("USERPROFILE")
		if len(home) != 0 {
			return NewFilePath(home)
		}
	} else {
		whoami, err := user.Current()
		if err == nil {
			if len(whoami.HomeDir) != 0 {
				return NewFilePath(whoami.HomeDir)
			}
		}
	}
	return NewFilePath(string(os.PathSeparator))
}
