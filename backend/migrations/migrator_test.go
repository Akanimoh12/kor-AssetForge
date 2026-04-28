package migrations_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/yourusername/kor-assetforge/migrations"
)

// openTestDB connects to the test database from DATABASE_URL env.
// If DATABASE_URL is not set the test is skipped.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "host=localhost user=postgres password=password dbname=assetforge_test port=5432 sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("cannot open test database: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		t.Skipf("test database not reachable: %v", err)
	}
	return db
}

func TestMigratorUpAndDown(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	m := migrations.New(db)

	// Apply all migrations
	if err := m.Up(); err != nil {
		t.Fatalf("Up() error: %v", err)
	}

	v, dirty, err := m.Version()
	if err != nil {
		t.Fatalf("Version() error: %v", err)
	}
	if dirty {
		t.Fatal("expected migration to be clean")
	}
	if v == 0 {
		t.Fatal("expected version > 0 after Up()")
	}

	// Roll back all (using 0 which we mapped to Down 1 step in our implementation, 
	// or we can loop to roll back everything)
	// For testing, let's just roll back once.
	if err := m.Down(1); err != nil {
		t.Fatalf("Down(1) error: %v", err)
	}

	v2, _, err := m.Version()
	if err != nil {
		t.Fatalf("Version() after Down error: %v", err)
	}
	if v2 >= v && v > 0 {
		t.Fatalf("expected version < %d after Down(1), got %d", v, v2)
	}
}

func TestMigratorIdempotent(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	m := migrations.New(db)

	if err := m.Up(); err != nil {
		t.Fatalf("first Up() error: %v", err)
	}
	// Running Up again should be a no-op
	if err := m.Up(); err != nil {
		t.Fatalf("second Up() error: %v", err)
	}
}
