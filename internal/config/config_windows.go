package config

import (
	"errors"
	"os"
)

func dir() (string, error) {
	d := os.Getenv("APPDATA")
	if d == "" {
		return "", errors.New("config: %APPDATA% must be defined")
	}
	return d, nil
}
