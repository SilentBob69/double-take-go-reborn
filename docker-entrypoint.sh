#!/bin/sh
# Startup-Skript für Double-Take-Go

# Verzeichnisse überprüfen
echo "Stelle sicher, dass Snapshot-Verzeichnisse existieren..."
mkdir -p /data/snapshots/frigate

echo "Starte Double-Take-Go mit Konfiguration aus $@"
# Start the application
exec /app/double-take "$@"
