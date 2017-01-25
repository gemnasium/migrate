// Package postgres implements the Driver interface.
package postgres

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate/direction"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type Driver struct {
	db *sqlx.DB
}

const tableName = "schema_migrations"
const txDisabledOption = "disable_ddl_transaction"

func (driver *Driver) Initialize(url string) error {
	db, err := sqlx.Open("postgres", url)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	driver.db = db

	return driver.ensureVersionTableExists()
}

func (driver *Driver) SetDB(db *sql.DB) {
	driver.db = sqlx.NewDb(db, "postgres")
}

func (driver *Driver) Close() error {
	return driver.db.Close()
}

func (driver *Driver) ensureVersionTableExists() error {
	// avoid DDL statements if possible for BDR (see #23)
	var c int
	driver.db.Get(&c, "SELECT count(*) FROM information_schema.tables WHERE table_name = $1;", tableName)
	if c > 0 {
		// table schema_migrations already exists, check if the schema is correct, ie: version is a bigint

		var dataType string
		err := driver.db.Get(&dataType, "SELECT data_type FROM information_schema.columns where table_name = $1 and column_name = 'version'", tableName)
		if err != nil {
			return err
		}

		if dataType == "bigint" {
			return nil
		}

		_, err = driver.db.Exec("ALTER TABLE " + tableName + " ALTER COLUMN version TYPE bigint USING version::bigint")
		return err
	}

	_, err := driver.db.Exec("CREATE TABLE IF NOT EXISTS " + tableName + " (version bigint not null primary key);")
	return err
}

func (driver *Driver) FilenameExtension() string {
	return "sql"
}

func (driver *Driver) Migrate(f file.File, pipe chan interface{}) {
	defer close(pipe)
	pipe <- f

	tx, err := driver.db.Begin()
	if err != nil {
		pipe <- err
		return
	}

	if f.Direction == direction.Up {
		if _, err := tx.Exec("INSERT INTO "+tableName+" (version) VALUES ($1)", f.Version); err != nil {
			pipe <- err
			if err := tx.Rollback(); err != nil {
				pipe <- err
			}
			return
		}
	} else if f.Direction == direction.Down {
		if _, err := tx.Exec("DELETE FROM "+tableName+" WHERE version=$1", f.Version); err != nil {
			pipe <- err
			if err := tx.Rollback(); err != nil {
				pipe <- err
			}
			return
		}
	}

	if err := f.ReadContent(); err != nil {
		pipe <- err
		return
	}

	if txDisabled(fileOptions(f.Content)) {
		_, err = driver.db.Exec(string(f.Content))
	} else {
		_, err = tx.Exec(string(f.Content))
	}

	if err != nil {
		pqErr := err.(*pq.Error)
		offset, err := strconv.Atoi(pqErr.Position)
		if err == nil && offset >= 0 {
			lineNo, columnNo := file.LineColumnFromOffset(f.Content, offset-1)
			errorPart := file.LinesBeforeAndAfter(f.Content, lineNo, 5, 5, true)
			pipe <- fmt.Errorf("%s %v: %s in line %v, column %v:\n\n%s", pqErr.Severity, pqErr.Code, pqErr.Message, lineNo, columnNo, string(errorPart))
		} else {
			pipe <- fmt.Errorf("%s %v: %s", pqErr.Severity, pqErr.Code, pqErr.Message)
		}

		if err := tx.Rollback(); err != nil {
			pipe <- err
		}
		return
	}

	if err := tx.Commit(); err != nil {
		pipe <- err
		return
	}
}

// Version returns the current migration version.
func (driver *Driver) Version() (file.Version, error) {
	var version file.Version
	err := driver.db.Get(&version, "SELECT version FROM "+tableName+" ORDER BY version DESC LIMIT 1")
	if err == sql.ErrNoRows {
		return version, nil
	}

	return version, err
}

// Versions returns the list of applied migrations.
func (driver *Driver) Versions() (file.Versions, error) {
	versions := file.Versions{}
	err := driver.db.Select(&versions, "SELECT version FROM "+tableName+" ORDER BY version DESC")
	return versions, err
}

// fileOptions returns the list of options extracted from the first line of the file content.
// Format: "-- <option1> <option2> <...>"
func fileOptions(content []byte) []string {
	firstLine := strings.Split(string(content), "\n")[0]
	if !strings.HasPrefix(firstLine, "-- ") {
		return []string{}
	}
	opts := strings.TrimPrefix(firstLine, "-- ")
	return strings.Split(opts, " ")
}

func txDisabled(opts []string) bool {
	for _, v := range opts {
		if v == txDisabledOption {
			return true
		}
	}
	return false
}

func init() {
	driver.RegisterDriver("postgres", &Driver{})
}
