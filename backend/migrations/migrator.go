package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"
	"time"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

// Migration represents a single migration version
type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

// MigrationRecord is the DB row stored in the schema_migrations table
type MigrationRecord struct {
	Version   int
	Name      string
	AppliedAt time.Time
}

// Migrator manages schema migrations
type Migrator struct {
	db *sql.DB
}

// New creates a new Migrator
func New(db *sql.DB) *Migrator {
	return &Migrator{db: db}
}

// Init creates the schema_migrations tracking table if it doesn't exist
func (m *Migrator) Init() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version   INTEGER PRIMARY KEY,
			name      TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	if err := m.Init(); err != nil {
		return fmt.Errorf("failed to init migrations table: %w", err)
	}

	pending, err := m.pending()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		log.Println("migrations: nothing to apply")
		return nil
	}

	for _, mg := range pending {
		log.Printf("migrations: applying %04d_%s ...", mg.Version, mg.Name)
		if err := m.apply(mg); err != nil {
			return fmt.Errorf("migration %04d_%s failed: %w", mg.Version, mg.Name, err)
		}
		log.Printf("migrations: applied  %04d_%s", mg.Version, mg.Name)
	}
	return nil
}

// Down rolls back n migrations (0 = all)
func (m *Migrator) Down(n int) error {
	if err := m.Init(); err != nil {
		return fmt.Errorf("failed to init migrations table: %w", err)
	}

	applied, err := m.applied()
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		log.Println("migrations: nothing to roll back")
		return nil
	}

	all, err := m.load()
	if err != nil {
		return err
	}
	byVersion := make(map[int]Migration, len(all))
	for _, mg := range all {
		byVersion[mg.Version] = mg
	}

	// Roll back in reverse order
	sort.Slice(applied, func(i, j int) bool { return applied[i].Version > applied[j].Version })
	if n > 0 && n < len(applied) {
		applied = applied[:n]
	}

	for _, rec := range applied {
		mg, ok := byVersion[rec.Version]
		if !ok {
			return fmt.Errorf("no migration file found for version %d", rec.Version)
		}
		log.Printf("migrations: rolling back %04d_%s ...", mg.Version, mg.Name)
		if err := m.rollback(mg); err != nil {
			return fmt.Errorf("rollback %04d_%s failed: %w", mg.Version, mg.Name, err)
		}
		log.Printf("migrations: rolled back %04d_%s", mg.Version, mg.Name)
	}
	return nil
}

// Status prints current migration state
func (m *Migrator) Status() ([]MigrationRecord, error) {
	if err := m.Init(); err != nil {
		return nil, err
	}
	return m.applied()
}

// Version returns the highest applied migration version, or 0 if none
func (m *Migrator) Version() (int, error) {
	applied, err := m.applied()
	if err != nil || len(applied) == 0 {
		return 0, err
	}
	return applied[len(applied)-1].Version, nil
}

func (m *Migrator) apply(mg Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(mg.UpSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		mg.Version, mg.Name,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Migrator) rollback(mg Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(mg.DownSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM schema_migrations WHERE version = $1`,
		mg.Version,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *Migrator) pending() ([]Migration, error) {
	all, err := m.load()
	if err != nil {
		return nil, err
	}

	applied, err := m.applied()
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[int]struct{}, len(applied))
	for _, rec := range applied {
		appliedSet[rec.Version] = struct{}{}
	}

	var pending []Migration
	for _, mg := range all {
		if _, done := appliedSet[mg.Version]; !done {
			pending = append(pending, mg)
		}
	}
	return pending, nil
}

func (m *Migrator) applied() ([]MigrationRecord, error) {
	rows, err := m.db.Query(
		`SELECT version, name, applied_at FROM schema_migrations ORDER BY version ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		if err := rows.Scan(&r.Version, &r.Name, &r.AppliedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// load reads all *.up.sql and *.down.sql files from the embedded sql/ directory
func (m *Migrator) load() ([]Migration, error) {
	entries, err := fs.ReadDir(sqlFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("failed to read sql directory: %w", err)
	}

	upFiles := make(map[string]string)
	downFiles := make(map[string]string)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := sqlFiles.ReadFile("sql/" + name)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(name, ".up.sql") {
			base := strings.TrimSuffix(name, ".up.sql")
			upFiles[base] = string(data)
		} else if strings.HasSuffix(name, ".down.sql") {
			base := strings.TrimSuffix(name, ".down.sql")
			downFiles[base] = string(data)
		}
	}

	var migrations []Migration
	for base, upSQL := range upFiles {
		version, migName, err := parseFilename(base)
		if err != nil {
			return nil, err
		}
		downSQL := downFiles[base]
		migrations = append(migrations, Migration{
			Version: version,
			Name:    migName,
			UpSQL:   upSQL,
			DownSQL: downSQL,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

// parseFilename extracts version and name from "0001_create_users"
func parseFilename(base string) (int, string, error) {
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid migration filename: %s", base)
	}
	var version int
	if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
		return 0, "", fmt.Errorf("invalid version in filename %s: %w", base, err)
	}
	return version, parts[1], nil
}
