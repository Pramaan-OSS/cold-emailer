package db

import (
	"embed"
	"errors"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrateConnectWait bounds how long boot waits for the server to accept the
// migration connection, e.g. while a rolling deploy's outgoing container still
// holds its slots.
const migrateConnectWait = 2 * time.Minute

// RunMigrations runs all pending database migrations.
// It uses embedded SQL files and is safe to call on every startup.
// Returns nil if migrations succeed or if there are no changes to apply.
func RunMigrations(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return errors.New("Database migrations could not be prepared.")
	}

	m, err := openMigrate(source, databaseURL)
	if err != nil {
		return errors.New("Database migration connection was not accepted. Check the saved database settings.")
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return errors.New("Database migrations failed. Check the database migration state before restarting.")
	}

	return nil
}

func openMigrate(sourceDrv source.Driver, databaseURL string) (*migrate.Migrate, error) {
	delay := 2 * time.Second
	deadline := time.Now().Add(migrateConnectWait)
	for {
		m, err := migrate.NewWithSourceInstance("iofs", sourceDrv, databaseURL)
		if err == nil || !serverBusy(err) || time.Now().Add(delay).After(deadline) {
			return m, err
		}
		log.Printf("Database is busy; retrying the migration connection in %s", delay)
		time.Sleep(delay)
		delay = min(delay*2, 15*time.Second)
	}
}

// serverBusy reports a refusal that clears on its own: no free connection
// slot (53300) or a server starting up or shutting down (57P03).
func serverBusy(err error) bool {
	var pe interface{ SQLState() string }
	if !errors.As(err, &pe) {
		return false
	}
	switch pe.SQLState() {
	case "53300", "57P03":
		return true
	}
	return false
}
