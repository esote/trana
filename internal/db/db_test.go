package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func TestTransactionRollback(t *testing.T) {
	db, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err = db.db.Exec(`CREATE TABLE "t" ("id" INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	testcases := map[string]func() error{
		"panic": func() error { panic("panic") },
		"error": func() error { return errors.New("error") },
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			err = db.Tx(context.Background(), func(tx *sql.Tx) error {
				if _, err := tx.Exec(`DROP TABLE "t"`); err != nil {
					t.Fatal(err)
				}
				return tc()
			})
			if err == nil {
				t.Fatal("transaction got nil; want err")
			}

			result, err := db.db.Exec(`INSERT INTO "t" ("id") VALUES (NULL)`)
			if err != nil {
				t.Fatal(err)
			}
			affected, err := result.RowsAffected()
			if err != nil {
				t.Fatal(err)
			}
			if affected != 1 {
				t.Fatalf("Inserted %d rows; want %d rows", affected, 1)
			}
		})
	}
}

func TestMigrate(t *testing.T) {
	const path = ":memory:"
	db, err := NewSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migrationSrc, err := iofs.New(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	migrationDB, err := sqlite3.WithInstance(db.db, &sqlite3.Config{
		DatabaseName: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("migrations", migrationSrc, path, migrationDB)
	if err != nil {
		t.Fatal(err)
	}

	// openDB() already did m.Up()
	if err = m.Down(); err != nil {
		t.Fatal(err)
	}
}
