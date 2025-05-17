# Hardware-Plattformen für Double-Take Go Reborn

Diese Dokumentation beschreibt die verschiedenen unterstützten Hardware-Plattformen und deren Konfiguration für Double-Take Go Reborn.

## Unterstützte Hardware-Plattformen

Double-Take Go Reborn ist für verschiedene Hardware-Architekturen optimiert und bietet auf jeder Plattform die bestmögliche Performance durch angepasste OpenCV-Konfiguration.

### CPU (Intel/AMD)

**Dockerfile**: `docker/cpu/Dockerfile`  
**Docker Compose**: `docker/cpu/docker-compose.yml`

Die CPU-Version ist die universellste Konfiguration und funktioniert auf allen Standard x86_64 Prozessoren ohne spezielle Hardware-Anforderungen.

#### Voraussetzungen
- Docker installiert
- Mindestens 2GB RAM, 4GB empfohlen
- x86_64 CPU (Intel oder AMD)

#### Starten
```bash
cd docker/cpu
docker compose up -d
cp ../config/examples/platforms/config-cpu.example.yaml ../config/config.yaml
# Editiere die Datei, um API-Keys und andere sensible Daten anzupassen
```

#### OpenCV-Konfiguration
Die CPU-Version wird mit Standard-OpenCV-Modulen gebaut und verwendet CPU-basierte Algorithmen für die Bildverarbeitung. Alle notwendigen Module wie ArUco (für interne Funktionen) sind aktiviert.

### NVIDIA GPU

**Dockerfile**: `docker/nvidia/Dockerfile`  
**Docker Compose**: `docker/nvidia/docker-compose.yml`

Diese Version ist für Systeme mit NVIDIA-Grafikkarten optimiert und nutzt CUDA für beschleunigte Bildverarbeitung.

#### Voraussetzungen
- Docker mit NVIDIA-Unterstützung installiert
- NVIDIA-Treiber installiert
- NVIDIA GPU mit CUDA-Unterstützung

#### Installation
1. Stelle sicher, dass Docker mit NVIDIA-GPU-Unterstützung konfiguriert ist:
   ```bash
   # Für neuere Docker-Versionen mit nvidia-container-toolkit
   sudo apt-get install -y nvidia-container-toolkit
   sudo systemctl restart docker
   ```

2. Starte den Container und verwende die entsprechende Konfiguration:
   ```bash
   cd docker/nvidia
   docker compose up -d
   cp ../config/examples/platforms/config-nvidia-gpu.example.yaml ../config/config.yaml
   # Editiere die Datei, um API-Keys und andere sensible Daten anzupassen
   ```

#### OpenCV-Konfiguration
Die NVIDIA-Version wird mit CUDA-Unterstützung gebaut und nutzt die GPU für beschleunigte Bildverarbeitung. Spezifische Optimierungen umfassen:
- CUDA-aktivierte OpenCV-Module
- CUDA-Unterstützung für DNN (Deep Neural Networks)
- NVIDIA Video Codec SDK für Hardware-beschleunigte Videodekodierung

### AMD GPU

**Dockerfile**: `docker/amd/Dockerfile`  
**Docker Compose**: `docker/amd/docker-compose.yml`

Diese Version ist für Systeme mit AMD-Grafikkarten optimiert und nutzt OpenCL für beschleunigte Bildverarbeitung.

#### Voraussetzungen
- Docker installiert
- AMD GPU mit OpenCL-Unterstützung
- ROCm-Treiber installiert

#### Installation
1. Stelle sicher, dass die ROCm-Treiber installiert sind:
   ```bash
   # Installation der ROCm-Treiber (Ubuntu-Beispiel)
   sudo apt-get update
   sudo apt-get install -y rocm-dev
   ```

2. Starte den Container und verwende die entsprechende Konfiguration:
   ```bash
   cd docker/amd
   docker compose up -d
   cp ../config/examples/platforms/config-amd-gpu.example.yaml ../config/config.yaml
   # Editiere die Datei, um API-Keys und andere sensible Daten anzupassen
   ```

#### OpenCV-Konfiguration
Die AMD-Version wird mit OpenCL-Unterstützung gebaut und nutzt die GPU für beschleunigte Bildverarbeitung. Spezifische Optimierungen umfassen:
- OpenCL-aktivierte OpenCV-Module
- OpenCL-Unterstützung für DNN

### Apple Silicon (M1/M2/M3)

**Dockerfile**: `docker/apple-silicon/Dockerfile`  
**Docker Compose**: `docker/apple-silicon/docker-compose.yml`

Diese Version ist speziell für Apple Silicon (ARM64) Prozessoren wie M1, M2 und M3 optimiert.

#### Voraussetzungen
- macOS auf Apple Silicon (M1/M2/M3)
- Docker Desktop für Apple Silicon installiert

#### Starten
```bash
cd docker/apple-silicon
docker compose up -d
cp ../config/examples/platforms/config-apple-silicon.example.yaml ../config/config.yaml
# Editiere die Datei, um API-Keys und andere sensible Daten anzupassen
```

#### OpenCV-Konfiguration
Die Apple-Silicon-Version wird mit ARM-spezifischen Optimierungen gebaut:
- NEON-Vektorisierung für verbesserte Performance
- Optimierungen für ARM64-Architektur

## Bekannte Probleme und Lösungen

### ArUco-Modul Abhängigkeiten

Die GoCV-Bibliothek benötigt das ArUco-Modul von OpenCV, auch wenn es nicht direkt von der Anwendung verwendet wird. Alle Docker-Konfigurationen sind so eingestellt, dass dieses Modul korrekt gebaut wird.

Wenn du eigene Docker-Builds erstellst, stelle sicher, dass du das ArUco-Modul nicht deaktivierst, sonst könnte es zu Build-Fehlern kommen.

### Plattformspezifische Builds

Die Docker-Konfigurationen sind plattformspezifisch optimiert. Versuche nicht, ein für eine Plattform gebautes Image auf einer anderen zu verwenden (z.B. ein NVIDIA-Image auf einem System ohne NVIDIA-GPU).

## Performance-Tipps

- **CPU-Version**: Bei der CPU-Version können Multi-Core-Prozessoren optimal ausgenutzt werden. Die Anzahl der verfügbaren Cores kann in der Konfiguration angepasst werden.
- **GPU-Versionen**: Die GPU-Versionen profitieren von modernen Grafikkarten mit viel VRAM. Bei begrenztem VRAM kann die Auflösung der Bilder in der Konfiguration reduziert werden.
- **Apple Silicon**: Die Apple Silicon Version nutzt speziell optimierte ARM-Vektorinstruktionen und bietet auf kompatiblen Macs die beste Performance.

## Erweiterte Konfiguration

Für detaillierte Informationen zur weiteren Konfiguration der einzelnen Plattformen, siehe die [Konfigurations-Dokumentation](CONFIGURATION.md).
