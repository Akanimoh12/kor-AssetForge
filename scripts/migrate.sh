#!/bin/bash
set -e

# Change to backend directory to run go commands
cd "$(dirname "$0")/../backend"

# Build the migration tool if it doesn't exist or always to be safe
echo "Building migration tool..."
go build -o ../scripts/migrate_bin ./cmd/migrate/main.go

# Run the tool with passed arguments
echo "Running migrations..."
../scripts/migrate_bin "$@"
