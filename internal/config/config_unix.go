//go:build !windows && !darwin
// +build !windows,!darwin

package config

import (
	"errors"
	"os"
	"path/filepath"
)

func dir() (string, error) {
	d := os.Getenv("XDG_CONFIG_HOME")
	if d == "" {
		if d = os.Getenv("HOME"); d == "" {
			return "", errors.New("config: $XDG_CONFIG_HOME or $HOME must be defined")
		}
		d = filepath.Join(d, ".config")
	}
	return d, nil
}
