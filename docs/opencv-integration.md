# OpenCV Integration für Double-Take-Go-Reborn

Diese Dokumentation beschreibt die Integration von OpenCV zur Personenerkennung in Double-Take-Go-Reborn, um die Effizienz und Genauigkeit der Gesichtserkennung mit CompreFace zu verbessern.

## Funktionsweise

Double-Take-Go-Reborn nutzt OpenCV als Vorfilter für Bilder, bevor diese zur Gesichtserkennung an CompreFace gesendet werden. Dies bietet mehrere Vorteile:

1. **Reduzierung unnötiger API-Aufrufe**: Nur Bilder mit erkannten Personen werden an CompreFace gesendet
2. **Verbesserte Genauigkeit**: Durch Fokussierung auf Bilder mit Personen werden Fehlalarme reduziert
3. **Optimale Ressourcennutzung**: Verbesserte Performance durch Nutzung von GPU-Beschleunigung (wenn verfügbar)

## Systemvoraussetzungen

### Basis-Anforderungen (CPU-Version)
- Docker und Docker Compose
- Mindestens 2GB RAM
- 20GB Festplattenspeicher

### Für NVIDIA GPU-Beschleunigung
- NVIDIA GPU mit CUDA-Unterstützung
- NVIDIA Container Toolkit (nvidia-docker)
- Aktuelle NVIDIA-Treiber

### Für AMD GPU-Beschleunigung
- AMD GPU mit OpenCL-Unterstützung
- ROCm-Treiber installiert
- ROCm-Docker-Container-Support

### Für Apple Silicon (M1/M2/M3)
- Apple Silicon Mac (M-Serie)
- Docker Desktop für Apple Silicon
- Mindestens 4GB freier RAM

## Installation und Konfiguration

### 1. Auswahl der richtigen Dockerfile-Variante

Je nach Ihrer Hardware-Plattform stehen verschiedene optimierte Docker-Container zur Verfügung:

- **Dockerfile.opencv**: Standard CPU-Version, funktioniert auf allen Plattformen
- **Dockerfile.opencv-cuda**: Optimiert für NVIDIA GPUs mit CUDA
- **Dockerfile.opencv-opencl**: Optimiert für AMD GPUs mit OpenCL
- **Dockerfile.opencv-arm64**: Optimiert für Apple Silicon (M-Serie)

### 2. Konfigurationsvorlage auswählen

Wählen Sie die passende Konfigurationsvorlage aus dem `config`-Verzeichnis:

```bash
# Für CPU-Version
cp config/config.opencv-cpu.yaml config/config.yaml

# Für NVIDIA GPU
cp config/config.opencv-nvidia.yaml config/config.yaml

# Für AMD GPU
cp config/config.opencv-amd.yaml config/config.yaml

# Für Apple Silicon
cp config/config.opencv-apple-silicon.yaml config/config.yaml
```

### 3. Docker Compose Konfiguration anpassen

Passen Sie die `docker-compose.yml` Datei an, um die richtige Dockerfile-Variante zu verwenden. Beispiele finden Sie in der `docker-compose.opencv.example.yml`.

### 4. CompreFace API-Key konfigurieren

Ersetzen Sie in der `config/config.yaml` den API-Key für CompreFace mit Ihrem eigenen:

```yaml
compreface:
  api_key: "Ihr-API-Key-hier"
```

### 5. System starten

```bash
docker-compose up -d
```

## Konfigurationsoptionen

Die OpenCV-Integration kann über folgende Parameter in der `config.yaml` konfiguriert werden:

```yaml
opencv:
  enabled: true              # OpenCV-Integration aktivieren/deaktivieren
  use_gpu: false             # GPU-Beschleunigung, wenn verfügbar
  person_detection:
    method: "hog"            # "hog" (CPU) oder "dnn" (für GPU)
    confidence_threshold: 0.5 # Erkennungsschwellenwert
    scale_factor: 1.05       # Skalierungsfaktor für Multi-Scale-Erkennung
    min_neighbors: 2         # Minimum benachbarter Erkennungen
    min_size_width: 64       # Minimale Breite einer Person
    min_size_height: 128     # Minimale Höhe einer Person
```

Für GPU-Beschleunigung können zusätzliche Parameter konfiguriert werden:

```yaml
opencv:
  # ... andere Parameter ...
  person_detection:
    # ... andere Parameter ...
    backend: "cuda"          # "default", "cuda" (NVIDIA), "opencl" (AMD)
    target: "cuda"           # "cpu", "cuda", "opencl"
```

## Fehlerbehebung

### OpenCV funktioniert nicht mit GPU

1. Überprüfen Sie, ob die GPU-Treiber korrekt installiert sind
2. Stellen Sie sicher, dass die richtigen Docker-Optionen für GPU-Zugriff konfiguriert sind
3. Setzen Sie `opencv.use_gpu: false` in der Konfiguration, um auf CPU-Modus zurückzufallen

### Container startet nicht

Prüfen Sie die Logs mit:
```bash
docker-compose logs double-take
```

### Leistungsprobleme

- Reduzieren Sie die Anzahl der gleichzeitigen Verarbeitungen über `processor.max_workers`
- Optimieren Sie den Erkennungsschwellenwert (`confidence_threshold`)
- Bei CPU-Nutzung: Verringern Sie die Bildgröße oder -auflösung

## Häufig gestellte Fragen

**F: Kann ich OpenCV deaktivieren und nur CompreFace nutzen?**  
A: Ja, setzen Sie `opencv.enabled: false` in Ihrer Konfiguration.

**F: Wie viel schneller ist die GPU-beschleunigte Version?**  
A: Je nach Hardware kann die GPU-Version 3-10x schneller sein als die CPU-Version.

**F: Funktioniert die Personenerkennung auch bei schlechten Lichtverhältnissen?**  
A: Die Erkennung funktioniert am besten bei guten Lichtverhältnissen. Bei schlechten Lichtverhältnissen sollte der Schwellenwert reduziert werden.

**F: Kann ich zwischen HOG und DNN wechseln ohne den Container neu zu bauen?**  
A: Ja, dies kann in der Konfigurationsdatei geändert werden, erfordert jedoch einen Neustart des Containers.
