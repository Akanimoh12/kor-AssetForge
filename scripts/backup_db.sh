#!/bin/bash
set -e

# Load environment variables
if [ -f backend/.env ]; then
    export $(grep -v '^#' backend/.env | xargs)
fi

DB_URL=${DATABASE_URL:-"postgres://postgres:password@localhost:5432/assetforge"}
BACKUP_DIR="backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/assetforge_${TIMESTAMP}.sql"

mkdir -p ${BACKUP_DIR}

echo "Backing up database to ${BACKUP_FILE}..."
# Using pg_dump. Assumes pg_dump is installed and reachable.
# If using Docker, you might need to run it inside the container.
if command -v pg_dump &> /dev/null; then
    pg_dump ${DB_URL} > ${BACKUP_FILE}
    echo "Backup completed: ${BACKUP_FILE}"
else
    echo "Warning: pg_dump not found. Skipping backup."
fi
