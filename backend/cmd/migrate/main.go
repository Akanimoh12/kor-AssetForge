package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/yourusername/kor-assetforge/migrations"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=password dbname=assetforge port=5432 sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	migrator := migrations.New(db)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: migrate <command> [arguments]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  up         Apply all pending migrations\n")
		fmt.Fprintf(os.Stderr, "  down [n]   Roll back the last n migrations (default: 1)\n")
		fmt.Fprintf(os.Stderr, "  version    Print the current migration version\n")
		fmt.Fprintf(os.Stderr, "  force <v>  Force the migration version to v\n")
		fmt.Fprintf(os.Stderr, "  seed       Apply seed data\n")
	}

	flag.Parse()
	args := flag.Args()

	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	command := args[0]

	switch command {
	case "up":
		if err := migrator.Up(); err != nil {
			log.Fatalf("up failed: %v", err)
		}
	case "down":
		n := 1
		if len(args) > 1 {
			var err error
			n, err = strconv.Atoi(args[1])
			if err != nil {
				log.Fatalf("invalid number of steps: %v", err)
			}
		}
		if err := migrator.Down(n); err != nil {
			log.Fatalf("down failed: %v", err)
		}
	case "version":
		v, dirty, err := migrator.Version()
		if err != nil {
			log.Fatalf("version failed: %v", err)
		}
		fmt.Printf("Current version: %d (dirty: %v)\n", v, dirty)
	case "force":
		if len(args) < 2 {
			log.Fatalf("force requires a version argument")
		}
		v, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("invalid version: %v", err)
		}
		if err := migrator.Force(v); err != nil {
			log.Fatalf("force failed: %v", err)
		}
	case "seed":
		if err := migrator.Seed(); err != nil {
			log.Fatalf("seed failed: %v", err)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}
}
