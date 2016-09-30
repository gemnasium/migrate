// Package phoenix implements the Driver interface.
package phoenix

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/Boostport/avatica"
	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate/direction"
	"strings"
)

type Driver struct {
	db *sql.DB
}

const tableName = "schema_migrations"

func (driver *Driver) Initialize(url string) error {

	urlWithoutScheme := strings.SplitN(url, "phoenix://", 2)

	if len(urlWithoutScheme) != 2 {
		return errors.New("invalid phoenix:// scheme")
	}

	db, err := sql.Open("avatica", "http://"+urlWithoutScheme[1])

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

func (driver *Driver) ensureVersionTableExists() error {
	_, err := driver.db.Exec("CREATE TABLE IF NOT EXISTS " + tableName + " (version bigint not null primary key)")
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
		if _, err := tx.Exec("INSERT INTO "+tableName+" (version) VALUES (?)", f.Version); err != nil {
			pipe <- err
			if err := tx.Rollback(); err != nil {
				pipe <- err
			}
			return
		}
	} else if f.Direction == direction.Down {
		if _, err := tx.Exec("DELETE FROM "+tableName+" WHERE version=?", f.Version); err != nil {
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

	// Phoenix does not support multiple statements, so we need to do this.
	sqlStmts := bytes.Split(f.Content, []byte(";"))

	for _, sqlStmt := range sqlStmts {

		sqlStmt = bytes.TrimSpace(sqlStmt)

		if len(sqlStmt) > 0 {
			if _, err := tx.Exec(string(sqlStmt)); err != nil {

				avaticaErr, isErr := err.(*avatica.ResponseError)

				if isErr {
					re, err := regexp.Compile(`at line ([0-9]+)`)

					if err != nil {
						pipe <- err
						if err := tx.Rollback(); err != nil {
							pipe <- err
						}
					}

					var lineNo int
					lineNoRe := re.FindStringSubmatch(avaticaErr.ErrorMessage)

					if len(lineNoRe) == 2 {
						lineNo, err = strconv.Atoi(lineNoRe[1])
					}

					if err == nil {

						// get white-space offset
						wsLineOffset := 0
						b := bufio.NewReader(bytes.NewBuffer(sqlStmt))
						for {
							line, _, err := b.ReadLine()
							if err != nil {
								break
							}
							if bytes.TrimSpace(line) == nil {
								wsLineOffset += 1
							} else {
								break
							}
						}

						message := avaticaErr.Error()
						message = re.ReplaceAllString(message, fmt.Sprintf("at line %v", lineNo+wsLineOffset))

						errorPart := file.LinesBeforeAndAfter(sqlStmt, lineNo, 5, 5, true)
						pipe <- errors.New(fmt.Sprintf("%s\n\n%s", message, string(errorPart)))
					} else {
						pipe <- errors.New(avaticaErr.Error())
					}

					if err := tx.Rollback(); err != nil {
						pipe <- err
					}

					return
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		pipe <- err
		return
	}
}

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

func init() {
	driver.RegisterDriver("phoenix", &Driver{})
}
