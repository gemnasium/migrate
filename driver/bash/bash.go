// Package bash implements the Driver interface.
package bash

import (
	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
)

type Driver struct {
}

func (driver *Driver) Initialize(url string) error {
	return nil
}

func (driver *Driver) Close() error {
	return nil
}

func (driver *Driver) FilenameExtension() string {
	return "sh"
}

func (driver *Driver) Migrate(f file.File, pipe chan interface{}) {
	defer close(pipe)
	pipe <- f
	return
}

// Version returns the current migration version.
func (driver *Driver) Version() (file.Version, error) {
	return file.Version(0), nil
}

// Versions returns the list of applied migrations.
func (driver *Driver) Versions() (file.Versions, error) {
	return file.Versions{0}, nil
}

func init() {
	driver.RegisterDriver("bash", &Driver{})
}
