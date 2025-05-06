#!/bin/bash

# Einfaches Lasttest-Skript für den Worker-Pool
# Sendet mehrere gleichzeitige API-Anfragen zum Verarbeiten von Bildern

# API-Endpunkt
API_URL="http://localhost:3000/api/process/image"

# Arbeitsverzeichnis für Testbilder
WORK_DIR="/tmp/double-take-test"
mkdir -p $WORK_DIR

# Funktion zum Erstellen eines Test-JPG-Bildes (minimal)
create_test_image() {
  local file="$1"
  convert -size 640x480 xc:white -font Helvetica -pointsize 20 \
    -fill black -annotate +20+50 "Test Image $(date +%s.%N)" \
    -fill blue -draw "rectangle 100,100 300,300" "$file"
}

# Prüfen ob ImageMagick installiert ist
if ! command -v convert &> /dev/null; then
  echo "Error: ImageMagick wird benötigt (convert tool)"
  exit 1
fi

# Anzahl der zu erstellenden Testanfragen
NUM_REQUESTS=20

echo "Erstelle $NUM_REQUESTS Testbilder..."
for i in $(seq 1 $NUM_REQUESTS); do
  TEST_FILE="$WORK_DIR/test_image_$i.jpg"
  create_test_image "$TEST_FILE"
  echo "Bild erstellt: $TEST_FILE"
done

echo "Starte Lasttest mit $NUM_REQUESTS parallelen Anfragen..."

# Funktion zum Senden einer Anfrage
send_request() {
  local file="$1"
  local id="$2"
  echo "Sende Anfrage $id mit Bild $file..."
  
  response=$(curl -s -X POST \
    -H "Content-Type: multipart/form-data" \
    -F "image=@$file" \
    -F "source=api_test" \
    $API_URL)
    
  echo "Antwort für $id: $response"
}

# Alle Anfragen parallel starten
for i in $(seq 1 $NUM_REQUESTS); do
  TEST_FILE="$WORK_DIR/test_image_$i.jpg"
  send_request "$TEST_FILE" "$i" &
done

# Auf alle Hintergrundprozesse warten
wait

echo "Lasttest abgeschlossen!"
