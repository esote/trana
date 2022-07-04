package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsDir(t *testing.T) {
	d, err := dir()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(d)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("config path %q is not a directory", d)
	}
}

func TestCreateDir(t *testing.T) {
	// Get temp name
	name := strings.ReplaceAll(t.TempDir(), string(filepath.Separator), "")
	d, err := dir()
	if err != nil {
		t.Fatal(err)
	}
	d = filepath.Join(d, name)
	if _, err = os.Stat(filepath.Join(d, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v; want %v", err, os.ErrNotExist)
	}

	// Create a new config directory
	d, err = Dir(name)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(d); base != name {
		t.Fatalf("got %q; want %q", base, name)
	}
	defer os.Remove(d)
	info, err := os.Stat(d)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("created config path %q is not a directory", d)
	}
}
