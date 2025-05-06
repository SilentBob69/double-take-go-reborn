# Double-Take Go

Eine Go-Implementierung inspiriert von [Double Take](https://github.com/jakowenko/double-take), einem System zur Gesichtserkennung und -verfolgung für Smart Homes und Überwachungskameras.

> **Hinweis**: Dieses Projekt befindet sich noch in einem frühen Entwicklungsstadium. Es ist funktional, hat aber möglicherweise noch Fehler und unvollständige Funktionen. Beiträge und Feedback sind willkommen!

## Danksagung

Dieses Projekt ist eine Neuimplementierung in Go und wurde stark inspiriert durch das hervorragende [Double Take](https://github.com/jakowenko/double-take) von [Jacob Kowenko](https://github.com/jakowenko). Die ursprüngliche Version verwendet Node.js und bietet einen größeren Funktionsumfang. Dieses Projekt strebt danach, ähnliche Funktionen in Go zu implementieren, hat aber noch nicht die vollständige Feature-Parität erreicht.

Wenn Sie nach einer bewährten und vollständigen Lösung suchen, empfehlen wir das Original-Projekt zu verwenden.

## Funktionen

- Integration mit CompreFace für die Gesichtserkennung
- MQTT-Integration für den Empfang von Ereignissen von Frigate NVR
- Echtzeit-Benachrichtigungen über Server-Sent Events (SSE)
- Webbasierte Benutzeroberfläche zur Verwaltung von Bildern und Gesichtern
- Automatische Bereinigung älterer Daten
- RESTful API für Integrationen mit anderen Systemen

## Anforderungen

- Docker und Docker Compose für die einfache Installation
- CompreFace-Instanz (als externer Dienst erreichbar unter der in der Konfiguration angegebenen URL)
- Optional: MQTT-Broker (als externer Dienst für die Integration mit Frigate)
- Optional: Frigate NVR (als externer Dienst zur Bereitstellung von Kamera-Events)

## Installation

1. Repository klonen:
   ```bash
   git clone https://github.com/SilentBob69/double-take-go-reborn.git
   cd double-take-go-reborn
   ```

2. Die Konfigurationsdatei erstellen:
   ```bash
   cp config/config.example.yaml config/config.yaml
   ```

3. Konfigurationsdatei anpassen (IP-Adressen, API-Schlüssel usw.):
   ```bash
   nano config/config.yaml
   ```

4. Starten der Anwendung mit Docker Compose:
   ```bash
   docker-compose up -d
   ```

5. Die Anwendung ist nun erreichbar unter:
   - Double-Take UI: http://localhost:3000

## Entwicklungsumgebung

Für die Entwicklung stellen wir eine spezielle Docker-Umgebung bereit:

1. Entwicklungsumgebung starten:
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. In den Container einsteigen:
   ```bash
   docker exec -it double-take-go-reborn-go-dev-1 /bin/bash
   ```

3. Anwendung im Container bauen:
   ```bash
   go build -o /app/bin/double-take /app/cmd/server/main.go
   ```

4. Anwendung im Container starten:
   ```bash
   /app/bin/double-take /app/config/config.yaml
   ```

5. Oder die Hilfsskripte verwenden:
   ```bash
   ./build.sh dev  # Startet die Entwicklungsumgebung
   ./build.sh run  # Startet die Produktionsumgebung
   ```

## Konfiguration

Die Hauptkonfigurationsdatei ist `config.yaml`. Wichtige Einstellungen sind:

- `server`: Hostnamen und Ports für den Server
- `compreface`: Verbindungsdetails für die externe CompreFace-Instanz
- `mqtt`: MQTT-Broker-Konfiguration für die Frigate-Integration
- `frigate`: Konfiguration für die Verbindung zu Frigate NVR
- `cleanup`: Einstellungen zur automatischen Datenlöschung

## Integration mit Frigate NVR

Um Double-Take mit Frigate zu verbinden:

1. MQTT-Integration in der Konfiguration aktivieren
2. Frigate so konfigurieren, dass Ereignisse an den MQTT-Broker gesendet werden
3. Im Frigate-Event-Topic die Snapshots für erkannte Personen aktivieren

## Technische Details

- Go 1.24 (siehe go.mod)
- Gin Web-Framework
- GORM als ORM mit SQLite-Datenbank
- Docker-basierte Deployment- und Entwicklungsumgebung

## Lizenz

[MIT](LICENSE)
