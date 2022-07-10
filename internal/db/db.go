package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migrateSqlite "github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	sqlite3 "github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("sqlite3_hook", &sqlite3.SQLiteDriver{
		ConnectHook: connectionHook,
	})
}

type DB interface {
	Tx(ctx context.Context, f func(tx *sql.Tx) error) error
	Close() error
}

var _ DB = (*SQLiteDB)(nil)

type SQLiteDB struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3_hook", path)
	if err != nil {
		return nil, err
	}
	if err = migrateDB(db, path); err != nil {
		db.Close()
		return nil, err
	}
	if err = setupDB(db); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteDB{db}, nil
}

func (db *SQLiteDB) Close() error {
	_, err := db.db.Exec(`PRAGMA optimize`)
	if err2 := db.db.Close(); err == nil {
		err = err2
	}
	return err
}

func (db *SQLiteDB) Tx(ctx context.Context, f func(tx *sql.Tx) error) (err error) {
	var tx *sql.Tx
	defer func() {
		if r := recover(); r != nil && err == nil {
			switch v := r.(type) {
			case error:
				err = v
			default:
				err = fmt.Errorf("%v", v)
			}
		}
		if tx != nil && err != nil {
			tx.Rollback()
		}
	}()
	tx, err = db.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	if err = f(tx); err != nil {
		return
	}
	err = tx.Commit()
	return err
}

const migrationDir = "migrations"

//go:embed migrations/*.sql
var migrations embed.FS

func migrateDB(db *sql.DB, path string) error {
	migrationSrc, err := iofs.New(migrations, migrationDir)
	if err != nil {
		return err
	}
	migrationDB, err := migrateSqlite.WithInstance(db, &migrateSqlite.Config{
		DatabaseName: path,
	})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance(migrationDir, migrationSrc, path, migrationDB)
	if err != nil {
		return err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// See: https://sqlite.org/security.html
func setupDB(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA trusted_schema = OFF`); err != nil {
		return err
	}

	var checkResult string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&checkResult); err != nil {
		return err
	}
	if checkResult != "ok" {
		return errors.New("database integrity check failed")
	}

	if _, err := db.Exec(`PRAGMA cell_size_check = ON`); err != nil {
		return err
	}

	if _, err := db.Exec(`PRAGMA mmap_size = 0`); err != nil {
		return err
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}

	if err := db.QueryRow(`PRAGMA foreign_key_check`).Scan(); !errors.Is(err, sql.ErrNoRows) {
		if err == nil {
			return errors.New("database contains foreign key errors")
		}
		return err
	}

	if _, err := db.Exec(`PRAGMA secure_delete = ON`); err != nil {
		return err
	}

	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return err
	}

	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		return err
	}

	if _, err := db.Exec(`PRAGMA synchronous = EXTRA`); err != nil {
		return err
	}

	// Changing from auto_vacuum=NONE to FULL requires VACUUM
	if _, err := db.Exec(`PRAGMA auto_vacuum = FULL; VACUUM`); err != nil {
		return err
	}

	return nil
}

// See: https://sqlite.org/security.html
func connectionHook(conn *sqlite3.SQLiteConn) error {
	// TODO: sqlite3_db_config  SQLITE_DBCONFIG_DEFENSIVE = 1

	conn.SetLimit(sqlite3.SQLITE_LIMIT_LENGTH, 1_000_000)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_SQL_LENGTH, 100_00)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_COLUMN, 100)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_EXPR_DEPTH, 10)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_COMPOUND_SELECT, 3)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_VDBE_OP, 25_000)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_FUNCTION_ARG, 8)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_ATTACHED, 1) // Documentation recommends 0, but VACUUM requires >0
	conn.SetLimit(sqlite3.SQLITE_LIMIT_LIKE_PATTERN_LENGTH, 50)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_VARIABLE_NUMBER, 10)
	conn.SetLimit(sqlite3.SQLITE_LIMIT_TRIGGER_DEPTH, 10)

	// TODO: sqlite3_db_config SQLITE_DBCONFIG_ENABLE_TRIGGER = 0
	// TODO: sqlite3_db_config SQLITE_DBCONFIG_ENABLE_VIEW = 0
	return nil
}
