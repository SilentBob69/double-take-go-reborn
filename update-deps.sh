#!/bin/bash
set -e

echo "Starte temporären Go-Container, um Abhängigkeiten zu aktualisieren..."
docker run --rm -v "$(pwd):/app" -w "/app" golang:1.20-alpine sh -c "
    echo 'Installiere benötigte Build-Tools...' && \
    apk add --no-cache git ca-certificates && \
    echo 'Aktualisiere Go-Module...' && \
    go mod tidy && \
    echo 'Lade Abhängigkeiten herunter...' && \
    go mod download
"

echo "Abhängigkeiten erfolgreich aktualisiert."
echo "Jetzt können Sie ./test.sh erneut ausführen, um die Anwendung zu bauen und zu starten."
