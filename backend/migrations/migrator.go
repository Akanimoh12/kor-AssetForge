package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

//go:embed sql/seed/*.sql
var seedFiles embed.FS

// Migrator manages schema migrations using golang-migrate
type Migrator struct {
	db *sql.DB
}

// New creates a new Migrator
func New(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// getMigrateInstance creates a new migrate instance using the embedded files
func (m *Migrator) getMigrateInstance() (*migrate.Migrate, error) {
	subFS, err := fs.Sub(sqlFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("failed to create sub filesystem: %w", err)
	}

	d, err := iofs.New(subFS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to create iofs driver: %w", err)
	}

	driver, err := postgres.WithInstance(m.db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	mgr, err := migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	return mgr, nil
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	mgr, err := m.getMigrateInstance()
	if err != nil {
		return err
	}
	defer mgr.Close()

	if err := mgr.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("migrations: up to date")
	return nil
}

// Down rolls back the last n migrations
func (m *Migrator) Down(n int) error {
	mgr, err := m.getMigrateInstance()
	if err != nil {
		return err
	}
	defer mgr.Close()

	if n <= 0 {
		// If n <= 0, we could interpret it as "down all" or just "down 1"
		// golang-migrate's Down() drops everything. 
		// Steps(-1) rolls back one.
		if err := mgr.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to rollback migration: %w", err)
		}
	} else {
		if err := mgr.Steps(-n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to rollback %d migrations: %w", n, err)
		}
	}

	log.Println("migrations: rollback successful")
	return nil
}

// Force sets the migration version manually (useful for fixing dirty migrations)
func (m *Migrator) Force(version int) error {
	mgr, err := m.getMigrateInstance()
	if err != nil {
		return err
	}
	defer mgr.Close()

	if err := mgr.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	log.Printf("migrations: forced version to %d", version)
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	mgr, err := m.getMigrateInstance()
	if err != nil {
		return 0, false, err
	}
	defer mgr.Close()

	version, dirty, err := mgr.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// Seed applies all SQL files from the sql/seed directory
func (m *Migrator) Seed() error {
	entries, err := seedFiles.ReadDir("sql/seed")
	if err != nil {
		return fmt.Errorf("failed to read seed directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		log.Printf("migrations: applying seed %s ...", entry.Name())
		content, err := seedFiles.ReadFile("sql/seed/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read seed file %s: %w", entry.Name(), err)
		}

		if _, err := m.db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute seed %s: %w", entry.Name(), err)
		}
		log.Printf("migrations: seed %s applied", entry.Name())
	}

	return nil
}
