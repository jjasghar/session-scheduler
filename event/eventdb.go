package event

import (
	//"database/sql"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
)

const codeSchemaVersion = 1

func isSqliteErrorCode(err error, queries ...error) bool {
	if err == nil {
		return false
	}
	sqliteErr, ok := err.(sqlite3.Error)
	if !ok {
		return false
	}
	for _, qerr := range queries {
		switch v := qerr.(type) {
		case sqlite3.ErrNo:
			if sqliteErr.Code == v {
				return true
			}
		case sqlite3.ErrNoExtended:
			if sqliteErr.ExtendedCode == v {
				return true
			}
		default:
			log.Printf("INTERNAL ERROR: isSqliteErrorCode passed invalid type %T", qerr)
		}
	}
	return false
}

func shouldRetry(err error) bool {
	return isSqliteErrorCode(err, sqlite3.ErrBusy, sqlite3.ErrLocked)
}

func errOrRetry(comment string, err error) error {
	if shouldRetry(err) {
		return err
	}
	return fmt.Errorf("%s: %v", comment, err)
}

func openDb(filename string) (*sqlx.DB, error) {

	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared", filename))
	if err != nil {
		return nil, err
	}

	// Check for existence of tables
	for {
		tx, err := db.Beginx()
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		var dbSchemaVersion int
		err = tx.Get(&dbSchemaVersion, "pragma user_version")
		if shouldRetry(err) {
			tx.Rollback()
			continue
		} else if err != nil {
			return nil, fmt.Errorf("Getting schema version: %v", err)
		}

		if dbSchemaVersion == 0 {
			err = initDb(tx)
			if shouldRetry(err) {
				tx.Rollback()
				continue
			} else if err != nil {
				return nil, fmt.Errorf("Initializing database: %v", err)
			}

			err = tx.Commit()
			if shouldRetry(err) {
				tx.Rollback()
				continue
			} else if err != nil {
				return nil, fmt.Errorf("Initializing database: %v", err)
			}
			break
		} else if dbSchemaVersion != codeSchemaVersion {
			return nil, fmt.Errorf("Wrong schema version (code %d, db %d)",
				codeSchemaVersion, dbSchemaVersion)
		}
		break
	}
	return db, nil
}

func initDb(ext sqlx.Ext) error {
	_, err := ext.Exec(fmt.Sprintf("pragma user_version=%d", codeSchemaVersion))
	if err != nil {
		return errOrRetry("Setting user_version", err)
	}

	_, err = ext.Exec(
		`CREATE TABLE event_locations(
    locationid   text primary key,
    locationname text not null,
    isplace      boolean not null,
    capacity     integer not null)`)

	if err != nil {
		return errOrRetry("Creating table event_location", err)
	}

	return nil
}
