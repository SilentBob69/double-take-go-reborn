# Double-Take Go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Status: Alpha](https://img.shields.io/badge/Status-Alpha-red.svg)]()
[![Docker](https://img.shields.io/badge/Docker-Required-blue.svg)]()
[![GitHub Stars](https://img.shields.io/github/stars/SilentBob69/double-take-go-reborn.svg?style=social)](https://github.com/SilentBob69/double-take-go-reborn/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/SilentBob69/double-take-go-reborn.svg?style=social)](https://github.com/SilentBob69/double-take-go-reborn/network/members)

*Read this in [English](README.en.md)*

Eine Go-Implementierung inspiriert von [Double Take](https://github.com/jakowenko/double-take), einem System zur Gesichtserkennung und -verfolgung für Smart Homes und Überwachungskameras.

> **Hinweis**: Dieses Projekt befindet sich noch in einem frühen Entwicklungsstadium. Es ist funktional, hat aber möglicherweise noch Fehler und unvollständige Funktionen. Beiträge und Feedback sind willkommen!

## Danksagung

Dieses Projekt ist eine Neuimplementierung in Go und wurde stark inspiriert durch das hervorragende [Double Take](https://github.com/jakowenko/double-take) von [Jacob Kowenko](https://github.com/jakowenko). Die ursprüngliche Version verwendet Node.js und bietet einen größeren Funktionsumfang. Dieses Projekt strebt danach, ähnliche Funktionen in Go zu implementieren, hat aber noch nicht die vollständige Feature-Parität erreicht.

Wenn Sie nach einer bewährten und vollständigen Lösung suchen, empfehlen wir das Original-Projekt zu verwenden.

## Funktionen

- Integration mit CompreFace für die Gesichtserkennung
  - Periodische Synchronisation der CompreFace-Subjekte (alle 15 Minuten)
  - Automatische Aktualisierung der lokalen Datenbank mit CompreFace-Daten
- MQTT-Integration für den Empfang von Ereignissen von Frigate NVR
- Home Assistant-Integration über MQTT für automatische Geräteerkennung und Statusaktualisierungen
- Echtzeit-Benachrichtigungen über Server-Sent Events (SSE)
- Moderne Toast-Benachrichtigungen für Systemereignisse und Benutzeraktionen
- Webbasierte Benutzeroberfläche zur Verwaltung von Bildern und Gesichtern
- Automatische Bereinigung älterer Daten
- RESTful API für Integrationen mit anderen Systemen
- Detaillierte Diagnoseseite mit System- und Datenbankstatistiken

## Anforderungen

- Docker und Docker Compose für die einfache Installation
- CompreFace-Instanz (als externer Dienst erreichbar unter der in der Konfiguration angegebenen URL)
- Optional: MQTT-Broker (als externer Dienst für die Integration mit Frigate und Home Assistant)
- Optional: Frigate NVR (als externer Dienst zur Bereitstellung von Kamera-Events)
- Optional: Home Assistant (für die automatische Integration der Erkennungsergebnisse)

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

## Docker Hub

Double-Take-Go-Reborn ist auch als fertiges Docker-Image auf Docker Hub verfügbar:

```bash
# Standard (latest)
docker pull silentbob69/double-take-go-reborn

# Mit explizitem latest Tag
docker pull silentbob69/double-take-go-reborn:latest
```

### Docker-Konfiguration

Die Anwendung kann mit dem Image von Docker Hub wie folgt gestartet werden:

```yaml
# docker-compose.yml Beispiel mit Docker Hub-Image
services:
  double-take:
    image: silentbob69/double-take-go-reborn:latest
    restart: unless-stopped
    volumes:
      - ./config:/config  # Konfigurationsdateien
      - ./data:/data      # Persistente Daten und Snapshots
    ports:
      - "3000:3000"       # Web-UI Port
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
```

### Volumes und Ports

Das Docker-Image verwendet folgende Volumes und Ports:

- **Volumes**:
  - `/config`: Enthält die Konfigurationsdateien (`config.yaml`)
  - `/data`: Speicherort für die Datenbank und Snapshot-Bilder
  
- **Ports**:
  - `3000`: Web-Benutzeroberfläche

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
  - `sync_interval_minutes`: Intervall in Minuten für die periodische CompreFace-Synchronisation (Standard: 15)
- `mqtt`: MQTT-Broker-Konfiguration für die Frigate-Integration
  - `homeassistant`: Einstellungen für die Home Assistant-Integration
- `frigate`: Konfiguration für die Verbindung zu Frigate NVR
- `cleanup`: Einstellungen zur automatischen Datenlöschung

### Beispielkonfiguration

```yaml
# config.yaml Beispiel
server:
  host: "0.0.0.0"           # Alle Interfaces binden
  port: 3000                # Web-UI Port
  snapshot_dir: "/data/snapshots"  # Wo Snapshots gespeichert werden
  snapshot_url: "/snapshots"      # URL-Pfad für Snapshots

log:
  level: "info"             # Log-Level (debug, info, warn, error)
  file: "/data/logs/double-take.log"  # Log-Datei

db:
  file: "/data/double-take.db"  # SQLite-Datenbank Pfad

compreface:
  enabled: true
  url: "http://10.100.0.3:8100"  # URL zur CompreFace-API
  recognition_api_key: "your_recognition_api_key"  # Von CompreFace generierter API-Key
  detection_api_key: "your_detection_api_key"      # Von CompreFace generierter API-Key
  det_prob_threshold: 0.8    # Erkennungsschwellenwert (0.0-1.0)
  sync_interval_minutes: 15   # Synchronisierungsintervall

mqtt:
  enabled: true              # MQTT aktivieren/deaktivieren
  broker: "192.168.0.55"     # MQTT-Broker Hostname/IP
  port: 1883                 # MQTT-Broker Port
  username: ""               # Optional: MQTT-Benutzername
  password: ""               # Optional: MQTT-Passwort
  client_id: "double-take-go"  # Client-ID für MQTT
  topic: "frigate/events"    # Topic für Frigate-Events

frigate:
  api_url: "http://192.168.0.55:5000"  # Frigate API URL
  url: "http://192.168.0.55:5000"      # Frigate Web-UI URL

cleanup:
  retention_days: 30         # Aufbewahrungsdauer für Bilder in Tagen
```

Kopieren Sie die Beispielkonfiguration in eine Datei namens `config.yaml` im `/config`-Verzeichnis und passen Sie sie an Ihre Bedürfnisse an.

## Neue Funktionen und Verbesserungen

- **Periodische CompreFace-Synchronisation**: Die Anwendung synchronisiert jetzt automatisch die Daten zwischen CompreFace und der lokalen Datenbank.
- **Toast-Benachrichtigungen**: Moderne, nicht-blockierende Benachrichtigungen für Systemereignisse und Benutzeraktionen.
- **Verbesserte Diagnostics-Seite**: Zeigt detaillierte Informationen über das System, die Datenbank und die CompreFace-Integration.
- **Bild-Neuverarbeitung**: Bilder können jetzt direkt aus der Benutzeroberfläche neu verarbeitet werden.
- **Vollständige Mehrsprachigkeit**: Komplette Unterstützung für Deutsch und Englisch in allen Teilen der Benutzeroberfläche mit Sprachauswahl und persistenter Speicherung der Spracheinstellung.
- **Verbesserte Navigation**: Fixierte Navigationsleiste für bessere Benutzerfreundlichkeit und Konsistenz über alle Seiten hinweg.
- **Scrollposition-Erhaltung**: Bei Sprachumschaltung bleibt die Scrollposition erhalten, was die Benutzerfreundlichkeit erhöht.

## API-Dokumentation

Double-Take Go stellt eine umfangreiche REST-API bereit, mit der andere Anwendungen mit dem System interagieren können. Eine vollständige Dokumentation der API-Endpunkte finden Sie hier:

- [API-Dokumentation (Deutsch)](docs/API.md)
- [API Documentation (English)](docs/API.en.md)

Die API ermöglicht die Steuerung aller wichtigen Funktionen des Systems, einschließlich der Bildverarbeitung, Identitätsverwaltung und Systemfunktionen.

## Feedback willkommen!

Wir freuen uns über Ihr Interesse an Double-Take Go Reborn und laden Sie herzlich ein, Feedback zu geben, Fragen zu stellen oder Verbesserungsvorschläge einzureichen. Ihre Beiträge helfen uns, das Projekt kontinuierlich zu verbessern!

- **Issues**: Haben Sie einen Fehler gefunden oder eine Idee für eine neue Funktion? [Erstellen Sie ein Issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new)!
- **Discussions**: Fragen zur Verwendung oder allgemeine Diskussionen finden im [Discussions-Bereich](https://github.com/SilentBob69/double-take-go-reborn/discussions) statt.
- **Pull Requests**: Code-Beiträge sind sehr willkommen! Schauen Sie sich unsere [Beitragsrichtlinien](CONTRIBUTING.md) an.

Jedes Feedback ist wertvoll, unabhängig davon, ob Sie ein erfahrener Entwickler sind oder das Projekt einfach nur ausprobieren möchten.

## Unterstützung

Wenn Ihnen dieses Projekt gefällt und Sie seine Entwicklung unterstützen möchten:

- **PayPal**: [Spendieren Sie mir ein Bier](https://www.paypal.com/donate/?hosted_button_id=6FTKYDXJ7R7ZL) über PayPal als Dankeschön.

Jede Unterstützung, egal ob finanziell oder durch Beiträge zum Projekt, wird sehr geschätzt und hilft dabei, Double-Take Go Reborn weiterzuentwickeln und zu verbessern.

## Zukünftige Pläne

- Verbesserung der Gesichtserkennungsgenauigkeit
- Erweiterung der Home Assistant-Integration
- Integration mit weiteren NVR-Systemen
- Mobile App-Integration
- Erweiterung der API-Funktionalität
