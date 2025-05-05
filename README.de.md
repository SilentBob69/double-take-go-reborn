# double-take-go-reborn

[![Status](https://img.shields.io/badge/status-early_development-orange.svg)](https://shields.io/)
[Read this in English](README.md)

**Wichtiger Hinweis:** Dieses Projekt befindet sich noch in einem sehr frühen Entwicklungsstadium. Funktionen können unvollständig sein, und es können Fehler auftreten.

## Beschreibung

`double-take-go-reborn` ist eine in Go geschriebene Anwendung zur Verarbeitung von Ereignissen (z. B. Bildern von Kameras), die über MQTT empfangen werden. Es integriert sich mit dem Gesichtserkennungsdienst [CompreFace](https://github.com/exadel-inc/CompreFace), um Personen in den Bildern zu erkennen und zu identifizieren. Die Anwendung bietet eine Web-API und potenziell eine Benutzeroberfläche zur Verwaltung und Anzeige der Ergebnisse.

Dieses Projekt ist eine Neuentwicklung ("Reborn") einer ähnlichen Konzeptanwendung, geschrieben in Go.

## Features (geplant/teilweise implementiert)

*   Empfang von Ereignissen über MQTT.
*   Integration mit CompreFace zur Gesichts-/Personenerkennung.
*   Speicherung von Ereignissen, Übereinstimmungen und Snapshots in einer SQLite-Datenbank.
*   Konfiguration über eine YAML-Datei (`config.yaml`).
*   Web-API (und potenziell UI) zum Abrufen von Daten und Verwalten von Identitäten.
*   Periodische Synchronisation von Identitäten mit CompreFace.
*   Automatisierte Bereinigung alter Daten und Snapshots.
*   Docker-Unterstützung für einfache Bereitstellung.

## Technologie-Stack

*   **Sprache:** Go
*   **Web Framework:** Gin
*   **Datenbank:** SQLite (über GORM)
*   **MQTT Client:** Paho MQTT Go Client
*   **Konfiguration:** Koanf
*   **Containerisierung:** Docker, Docker Compose
*   **Gesichtserkennung (extern):** CompreFace

## Erste Schritte

### Voraussetzungen

*   Docker und Docker Compose müssen installiert sein.
*   Eine laufende Instanz von CompreFace (entweder lokal oder remote).
*   Ein MQTT-Broker.

### Konfiguration

1.  Kopieren oder benennen Sie die Beispielkonfiguration `config/config.example.yaml` nach `/config/config.yaml` (oder mounten Sie Ihre eigene Konfigurationsdatei an diesen Pfad im Container).
2.  Passen Sie die `config.yaml` an Ihre Umgebung an, insbesondere die Verbindungsdaten für MQTT und CompreFace sowie die Verzeichnispfade.

### Ausführen mit Docker Compose

Die einfachste Methode, die Anwendung zu starten, ist die Verwendung der mitgelieferten `docker-compose.yml`-Datei:

```bash
docker-compose up -d
```

Dies startet den `double-take-go-reborn`-Dienst. Stellen Sie sicher, dass Ihr MQTT-Broker und CompreFace ebenfalls laufen und erreichbar sind.

## Status

Wie bereits erwähnt, befindet sich das Projekt in der **frühen Entwicklung**. Änderungen an der API und der Funktionalität sind wahrscheinlich.

## Lizenz

Dieses Projekt steht unter der MIT-Lizenz. Siehe die [LICENSE](LICENSE)-Datei für Details.
