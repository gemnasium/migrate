// Package mysql implements the Driver interface.
package mysql

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gemnasium/migrate/driver"
	"github.com/gemnasium/migrate/file"
	"github.com/gemnasium/migrate/migrate/direction"
	"github.com/go-sql-driver/mysql"
)

type Driver struct {
	db *sql.DB
}

const tableName = "schema_migrations"

func (driver *Driver) Initialize(url string) error {
	urlWithoutScheme := strings.SplitN(url, "mysql://", 2)
	if len(urlWithoutScheme) != 2 {
		return errors.New("invalid mysql:// scheme")
	}

	// check if env vars vor mysql ssl connection are set and if yes use them
	if os.Getenv("MYSQL_SERVER_CA") != "" && os.Getenv("MYSQL_CLIENT_KEY") != "" && os.Getenv("MYSQL_CLIENT_CERT") != "" {
		rootCertPool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(os.Getenv("MYSQL_SERVER_CA"))
		if err != nil {
			return err
		}

		if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
			return errors.New("Failed to append PEM")
		}

		clientCert := make([]tls.Certificate, 0, 1)
		certs, err := tls.LoadX509KeyPair(os.Getenv("MYSQL_CLIENT_CERT"), os.Getenv("MYSQL_CLIENT_KEY"))
		if err != nil {
			return err
		}

		clientCert = append(clientCert, certs)
		mysql.RegisterTLSConfig("custom", &tls.Config{
			RootCAs:            rootCertPool,
			Certificates:       clientCert,
			InsecureSkipVerify: true,
		})

		urlWithoutScheme[1] += "&tls=custom"
	}

	db, err := sql.Open("mysql", urlWithoutScheme[1])
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
	_, err := driver.db.Exec("CREATE TABLE IF NOT EXISTS " + tableName + " (version bigint not null primary key);")

	if err != nil {
		return err
	}
	r := driver.db.QueryRow("SELECT data_type FROM information_schema.columns where table_name = ? and column_name = 'version'", tableName)
	dataType := ""
	if err := r.Scan(&dataType); err != nil {
		return err
	}
	if dataType != "int" {
		return nil
	}
	_, err = driver.db.Exec("ALTER TABLE " + tableName + " MODIFY version bigint")
	return err
}

func (driver *Driver) FilenameExtension() string {
	return "sql"
}

func (driver *Driver) Migrate(f file.File, pipe chan interface{}) {
	defer close(pipe)
	pipe <- f

	// http://go-database-sql.org/modifying.html, Working with Transactions
	// You should not mingle the use of transaction-related functions such as Begin() and Commit() with SQL statements such as BEGIN and COMMIT in your SQL code.
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
		if _, err := tx.Exec("DELETE FROM "+tableName+" WHERE version = ?", f.Version); err != nil {
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

	// TODO this is not good! unfortunately there is no mysql driver that
	// supports multiple statements per query.
	sqlStmts := bytes.Split(f.Content, []byte(";"))

	for _, sqlStmt := range sqlStmts {
		sqlStmt = bytes.TrimSpace(sqlStmt)
		if len(sqlStmt) > 0 {
			if _, err := tx.Exec(string(sqlStmt)); err != nil {
				mysqlErr, isErr := err.(*mysql.MySQLError)

				if isErr {
					re, err := regexp.Compile(`at line ([0-9]+)$`)
					if err != nil {
						pipe <- err
						if err := tx.Rollback(); err != nil {
							pipe <- err
						}
					}

					var lineNo int
					lineNoRe := re.FindStringSubmatch(mysqlErr.Message)
					if len(lineNoRe) == 2 {
						lineNo, err = strconv.Atoi(lineNoRe[1])
					}
					if err == nil {

						// get white-space offset
						// TODO this is broken, because we use sqlStmt instead of f.Content
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

						message := mysqlErr.Error()
						message = re.ReplaceAllString(message, fmt.Sprintf("at line %v", lineNo+wsLineOffset))

						errorPart := file.LinesBeforeAndAfter(sqlStmt, lineNo, 5, 5, true)
						pipe <- errors.New(fmt.Sprintf("%s\n\n%s", message, string(errorPart)))
					} else {
						pipe <- errors.New(mysqlErr.Error())
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

// Version returns the current migration version.
func (driver *Driver) Version() (file.Version, error) {
	var version file.Version
	err := driver.db.QueryRow("SELECT version FROM " + tableName + " ORDER BY version DESC").Scan(&version)
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

func init() {
	driver.RegisterDriver("mysql", &Driver{})
}
