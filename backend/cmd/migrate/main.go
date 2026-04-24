package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/yourusername/kor-assetforge/migrations"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=password dbname=assetforge port=5432 sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	m := migrations.New(db)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: migrate <command> [args]

Commands:
  up              Apply all pending migrations
  down [n]        Roll back n migrations (default: 1; use 0 for all)
  status          Show applied migrations
  version         Show current migration version
`)
	}
	flag.Parse()

	cmd := flag.Arg(0)
	switch cmd {
	case "up":
		if err := m.Up(); err != nil {
			log.Fatalf("migrate up: %v", err)
		}

	case "down":
		n := 1
		if arg := flag.Arg(1); arg != "" {
			v, err := strconv.Atoi(arg)
			if err != nil {
				log.Fatalf("invalid count %q: %v", arg, err)
			}
			n = v
		}
		if err := m.Down(n); err != nil {
			log.Fatalf("migrate down: %v", err)
		}

	case "status":
		records, err := m.Status()
		if err != nil {
			log.Fatalf("migrate status: %v", err)
		}
		if len(records) == 0 {
			fmt.Println("No migrations applied.")
			return
		}
		fmt.Printf("%-8s %-40s %s\n", "VERSION", "NAME", "APPLIED AT")
		fmt.Println(string(make([]byte, 70)))
		for _, r := range records {
			fmt.Printf("%-8d %-40s %s\n", r.Version, r.Name, r.AppliedAt.Format("2006-01-02 15:04:05"))
		}

	case "version":
		v, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("current version: %d\n", v)

	default:
		flag.Usage()
		os.Exit(1)
	}
}
