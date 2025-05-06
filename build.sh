#!/bin/bash
set -e

# Hilfe anzeigen
function show_help() {
  echo "Double-Take-Go Build-Helper"
  echo ""
  echo "Verwendung: $0 [command]"
  echo ""
  echo "Commands:"
  echo "  build      Kompiliert die Anwendung im Container"
  echo "  run        Führt die Anwendung im Container aus"
  echo "  dev        Startet die Entwicklungsumgebung"
  echo "  test       Führt Tests im Container aus"
  echo "  clean      Bereinigt Build-Artefakte"
  echo "  stop       Stoppt alle laufenden Container"
  echo "  exec       Führt einen Befehl im Go-Container aus"
  echo ""
  echo "Beispiel: $0 build"
  exit 1
}

# Parameter prüfen
if [ $# -lt 1 ]; then
  show_help
fi

# Stellt sicher, dass der go-dev Container läuft
ensure_dev_container_running() {
  if ! docker-compose -f docker-compose.dev.yml ps | grep -q "go-dev.*Up"; then
    echo "Starte Entwicklungscontainer..."
    docker-compose -f docker-compose.dev.yml up -d go-dev
  fi
}

# Führt einen Befehl im Container aus
run_in_container() {
  ensure_dev_container_running
  docker-compose -f docker-compose.dev.yml exec go-dev sh -c "$1"
}

# Hauptlogik
case "$1" in
  build)
    echo "Kompilieren der Anwendung im Container..."
    run_in_container "cd /app && CGO_ENABLED=0 GOOS=linux go build -o double-take ./cmd/server"
    echo "Build abgeschlossen: ./double-take"
    ;;
    
  run)
    echo "Starte Anwendung im Container..."
    run_in_container "cd /app && go run cmd/server/main.go"
    ;;
    
  dev)
    echo "Starte Entwicklungsumgebung..."
    docker-compose -f docker-compose.dev.yml up -d
    echo "Entwicklungsumgebung läuft. Komponenten:"
    echo "- Go-Entwicklungscontainer (go-dev)"
    echo "- CompreFace API (http://localhost:8000)"
    echo "- CompreFace UI (http://localhost:8080)"
    echo "- MQTT-Broker (localhost:1883)"
    echo "- PostgreSQL (localhost:5432)"
    echo ""
    echo "Für eine Shell im Go-Container: ./build.sh exec sh"
    ;;
    
  test)
    echo "Führe Tests im Container aus..."
    run_in_container "cd /app && go test -v ./..."
    ;;
    
  clean)
    echo "Bereinige Build-Artefakte..."
    run_in_container "cd /app && rm -f double-take"
    echo "Bereinigung abgeschlossen."
    ;;
    
  stop)
    echo "Stoppe alle Container..."
    docker-compose -f docker-compose.dev.yml down
    echo "Alle Container gestoppt."
    ;;
    
  exec)
    shift
    if [ $# -lt 1 ]; then
      echo "Fehler: Kein Befehl angegeben."
      echo "Verwendung: $0 exec [command]"
      exit 1
    fi
    
    ensure_dev_container_running
    echo "Führe aus: $@"
    docker-compose -f docker-compose.dev.yml exec go-dev $@
    ;;
    
  *)
    show_help
    ;;
esac
