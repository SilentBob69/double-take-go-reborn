# Double-Take Go Reborn

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

## 📘 Für Benutzer

### Schnellstart

```bash
# 1. Repository klonen
git clone https://github.com/SilentBob69/double-take-go-reborn.git
cd double-take-go-reborn

# 2. Wähle die passende Version für deine Hardware:
#    - CPU: Standard für Intel/AMD Prozessoren ohne GPU-Beschleunigung
#    - NVIDIA: Für NVIDIA GPU-Beschleunigung (erfordert nvidia-docker)
#    - AMD: Für AMD GPU-Beschleunigung mit OpenCL (erfordert ROCm)
#    - Apple-Silicon: Für M1/M2/M3 Chips
cd docker/cpu            # oder: nvidia, amd, apple-silicon

# 3. Starte den Container
docker compose up -d
```

Die Anwendung ist nun erreichbar unter:
- Double-Take UI: http://localhost:3000

### Hardware-Unterstützung

Double-Take Go Reborn unterstützt folgende Hardwarekonfigurationen:

| Plattform | Verzeichnis | Beschreibung | Anforderungen |
|-----------|-------------|--------------|---------------|
| CPU | `docker/cpu/` | Standard x86_64 Prozessoren | Docker |
| NVIDIA GPU | `docker/nvidia/` | CUDA-Beschleunigung | Docker mit NVIDIA Support |
| AMD GPU | `docker/amd/` | OpenCL-Beschleunigung | Docker mit ROCm Support |
| Apple Silicon | `docker/apple-silicon/` | Metal-Optimierung für M1/M2/M3 | Docker für ARM64 |

Jede Plattform bietet optimierte OpenCV-Integration für die jeweilige Hardware und ist mit einer passenden `docker-compose.yml` konfiguriert. Die Dateien enthalten ausführliche Kommentare zur Konfiguration.

### Dokumentation

- 🇩🇪 **Deutsch**
  - [Installation](docs/INSTALLATION.md)
  - [Konfiguration](docs/CONFIGURATION.md)
  - [Hardware-Plattformen](docs/PLATFORMS.md)
  - [Fehlersuche](docs/TROUBLESHOOTING.md)
  - [API-Dokumentation](docs/API.md)
  
- 🇬🇧 **English**
  - [Installation](docs/INSTALLATION.en.md)
  - [Configuration](docs/CONFIGURATION.en.md)
  - [Hardware Platforms](docs/PLATFORMS.en.md)
  - [Troubleshooting](docs/TROUBLESHOOTING.en.md) 
  - [API Documentation](docs/API.en.md)

### Funktionen

- Integration mit CompreFace für die Gesichtserkennung
  - Periodische Synchronisation der CompreFace-Subjekte (alle 15 Minuten)
  - Automatische Aktualisierung der lokalen Datenbank mit CompreFace-Daten
- OpenCV-Integration für effiziente Personenerkennung
  - Vorfilterung von Bildern zur Reduzierung unnötiger API-Aufrufe an CompreFace
  - GPU-Beschleunigung auf unterstützter Hardware (NVIDIA, AMD, Apple Silicon)
  - Konfigurierbare Parameter für optimale Erkennungsleistung
- MQTT-Integration für den Empfang von Ereignissen von Frigate NVR
- Home Assistant-Integration über MQTT für automatische Geräteerkennung und Statusaktualisierungen
- Echtzeit-Benachrichtigungen über Server-Sent Events (SSE)
- Moderne Toast-Benachrichtigungen für Systemereignisse und Benutzeraktionen
- Webbasierte Benutzeroberfläche zur Verwaltung von Bildern und Gesichtern
- Automatische Bereinigung älterer Daten
- RESTful API für Integrationen mit anderen Systemen
- Detaillierte Diagnoseseite mit System- und Datenbankstatistiken
- Vollständige Mehrsprachigkeit (Deutsch und Englisch)

### Unterstützte Plattformen

- **CPU-Version**: Funktioniert auf allen Plattformen, geringste Systemanforderungen
- **NVIDIA GPU-Version**: Optimierte Performance durch CUDA-Beschleunigung
- **AMD GPU-Version**: OpenCL-beschleunigte Variante
- **Apple Silicon-Version**: Speziell optimiert für M1/M2/M3-Prozessoren

## 🛠 Für Entwickler

### Dokumentation

- 🇩🇪 **Deutsch**
  - [Entwicklungsumgebung](docs/DEVELOPMENT.md)
  - [Architektur](docs/ARCHITECTURE.md)
  - [Testen](docs/TESTING.md)
  - [Beitragsrichtlinien](CONTRIBUTING.md)
  
- 🇬🇧 **English**
  - [Development Environment](docs/DEVELOPMENT.en.md)
  - [Architecture](docs/ARCHITECTURE.en.md)
  - [Testing](docs/TESTING.en.md)
  - [Contribution Guidelines](CONTRIBUTING.en.md)

### Projektstruktur

Die neue Projektstruktur ist so organisiert:

```
/
├── docker/                   # Docker-Konfigurationen für alle Plattformen
│   ├── cpu/                  # CPU-Version
│   │   ├── Dockerfile        # Dockerfile für CPU-Version
│   │   └── docker-compose.yml # Docker Compose für CPU-Version
│   ├── nvidia/               # NVIDIA GPU-Version
│   ├── amd/                  # AMD GPU-Version 
│   └── apple-silicon/        # Apple Silicon-Version
├── config/                   # Konfigurationsdateien
│   ├── config.yaml           # Hauptkonfiguration
│   ├── config.example.yaml   # Beispielkonfiguration (ohne sensible Daten)
│   ├── platforms/            # Plattformspezifische Konfigurationen (aktiv genutzte)
│   │   ├── config-cpu.yaml              # Konfiguration für CPU
│   │   ├── config-nvidia-gpu.yaml       # Konfiguration für NVIDIA
│   │   ├── config-amd-gpu.yaml          # Konfiguration für AMD
│   │   └── config-apple-silicon.yaml    # Konfiguration für Apple Silicon
│   └── examples/             # Beispielkonfigurationen (ohne sensible Daten)
│       └── platforms/        # Plattformspezifische Beispielkonfigurationen
│           ├── config-cpu.example.yaml          # Beispiel für CPU
│           ├── config-nvidia-gpu.example.yaml   # Beispiel für NVIDIA
│           ├── config-amd-gpu.example.yaml      # Beispiel für AMD
│           └── config-apple-silicon.example.yaml # Beispiel für Apple Silicon
└── docs/                     # Dokumentation
    ├── INSTALLATION.md       # Deutsche Installationsanleitung
    ├── INSTALLATION.en.md    # Englische Installationsanleitung
    └── ...                   # Weitere Dokumentationsdateien
```

### Docker-Entwicklungsumgebung

```bash
# Entwicklungsumgebung starten
docker-compose -f docker-compose.yml up -d
```

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
