# Double-Take Go

Eine Go-Implementierung von [Double Take](https://github.com/jakowenko/double-take), einem System zur Gesichtserkennung und -verfolgung für Smart Homes und Überwachungskameras.

## Funktionen

- Integration mit CompreFace für die Gesichtserkennung
- MQTT-Integration für den Empfang von Ereignissen von Frigate NVR
- Echtzeit-Benachrichtigungen über Server-Sent Events (SSE)
- Webbasierte Benutzeroberfläche zur Verwaltung von Bildern und Gesichtern
- Automatische Bereinigung älterer Daten
- RESTful API für Integrationen mit anderen Systemen

## Anforderungen

- Docker und Docker Compose für die einfache Installation
- CompreFace (wird automatisch über docker-compose bereitgestellt)
- Optional: MQTT-Broker (wird mit Mosquitto bereitgestellt)

## Installation

1. Repository klonen:
   ```bash
   git clone https://github.com/username/double-take-go-reborn.git
   cd double-take-go-reborn
   ```

2. Die Konfigurationsdatei erstellen:
   ```bash
   cp config/config.example.yaml config/config.yaml
   ```

3. Konfigurationsdatei anpassen:
   ```bash
   nano config/config.yaml
   ```

4. Starten der Anwendung mit Docker Compose:
   ```bash
   docker-compose up -d
   ```

5. Die Anwendung ist nun erreichbar unter:
   - Double-Take UI: http://localhost:3000
   - CompreFace UI: http://localhost:8000

## Konfiguration

Die Hauptkonfigurationsdatei ist `config.yaml`. Wichtige Einstellungen sind:

- `server`: Hostnamen und Ports für den Server
- `compreface`: Verbindungsdetails für CompreFace
- `mqtt`: MQTT-Broker-Konfiguration für die Frigate-Integration
- `cleanup`: Einstellungen zur automatischen Datenlöschung

## Integration mit Frigate NVR

Um Double-Take mit Frigate zu verbinden:

1. MQTT-Integration in der Konfiguration aktivieren
2. Frigate so konfigurieren, dass Ereignisse an den MQTT-Broker gesendet werden
3. Im Frigate-Event-Topic die Snapshots für erkannte Personen aktivieren

## Entwicklung

### Voraussetzungen

- Go 1.19 oder höher
- Git

### Lokale Entwicklung

1. Repository klonen
2. Abhängigkeiten installieren:
   ```bash
   go mod download
   ```
3. Anwendung starten:
   ```bash
   go run cmd/server/main.go ./config/config.yaml
   ```

## Lizenz

[MIT](LICENSE)
