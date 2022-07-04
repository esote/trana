package trana

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/mattn/go-sqlite3"
)

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	// TODO: enable foreign keys, do integrity checks, set defensive config, etc
	if err = migrateDB(db, path); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

const migrationDir = "migrations"

//go:embed migrations/*.sql
var migrations embed.FS

func migrateDB(db *sql.DB, path string) error {
	migrationSrc, err := iofs.New(migrations, migrationDir)
	if err != nil {
		return err
	}
	migrationDB, err := sqlite3.WithInstance(db, &sqlite3.Config{
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

func tx(db *sql.DB, ctx context.Context, f func(tx *sql.Tx) error) (err error) {
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
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		return
	}

	if err = f(tx); err != nil {
		return
	}
	err = tx.Commit()
	return err
}
