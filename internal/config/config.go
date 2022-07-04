package config

import (
	"errors"
	"os"
	"path/filepath"
)

func Dir(name string) (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	d = filepath.Join(d, name)

	if err = os.Mkdir(d, 0o755); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
		err = nil
	}

	return d, nil
}
