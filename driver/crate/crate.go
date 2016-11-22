// Package crate implements a driver for the Crate.io database
package crate

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate/direction"
	_ "github.com/herenow/go-crate"
)

func init() {
	driver.RegisterDriver("crate", &Driver{})
}

type Driver struct {
	db *sql.DB
}

const tableName = "schema_migrations"

func (driver *Driver) Initialize(url string) error {
	url = strings.Replace(url, "crate", "http", 1)
	db, err := sql.Open("crate", url)
	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}
	driver.db = db

	if err := driver.ensureVersionTableExists(); err != nil {
		return err
	}
	return nil
}

func (driver *Driver) Close() error {
	if err := driver.db.Close(); err != nil {
		return err
	}
	return nil
}

func (driver *Driver) FilenameExtension() string {
	return "sql"
}

// Version returns the current migration version.
func (driver *Driver) Version() (file.Version, error) {
	var version file.Version
	err := driver.db.QueryRow("SELECT version FROM " + tableName + " ORDER BY version DESC LIMIT 1").Scan(&version)
	switch {
	case err == sql.ErrNoRows:
		return 0, nil
	case err != nil:
		return 0, err
	default:
		return version, nil
	}
}

// Versions returns the list of applied migrations.
func (driver *Driver) Versions() (file.Versions, error) {
	versions := file.Versions{}

	rows, err := driver.db.Query("SELECT version FROM " + tableName + " ORDER BY version DESC")
	if err != nil {
		return versions, err
	}
	defer rows.Close()
	for rows.Next() {
		var version file.Version
		err := rows.Scan(&version)
		if err != nil {
			return versions, err
		}
		versions = append(versions, version)
	}
	err = rows.Err()
	return versions, err
}

func (driver *Driver) Migrate(f file.File, pipe chan interface{}) {
	defer close(pipe)
	pipe <- f

	if err := f.ReadContent(); err != nil {
		pipe <- err
		return
	}

	lines := splitContent(string(f.Content))
	for _, line := range lines {
		_, err := driver.db.Exec(line)
		if err != nil {
			pipe <- err
			return
		}
	}

	if f.Direction == direction.Up {
		if _, err := driver.db.Exec("INSERT INTO "+tableName+" (version) VALUES (?)", f.Version); err != nil {
			pipe <- err
			return
		}
	} else if f.Direction == direction.Down {
		if _, err := driver.db.Exec("DELETE FROM "+tableName+" WHERE version=?", f.Version); err != nil {
			pipe <- err
			return
		}
	}
}

func splitContent(content string) []string {
	lines := strings.Split(content, ";")
	resultLines := make([]string, 0, len(lines))
	for i, line := range lines {
		line = strings.Replace(lines[i], ";", "", -1)
		line = strings.TrimSpace(line)
		if line != "" {
			resultLines = append(resultLines, line)
		}
	}
	return resultLines
}

func (driver *Driver) ensureVersionTableExists() error {
	if _, err := driver.db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version LONG PRIMARY KEY)", tableName)); err != nil {
		return err
	}
	return nil
}
