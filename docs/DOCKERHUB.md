# Installation mit Docker Hub

Diese Anleitung beschreibt, wie Sie Double-Take Go Reborn direkt über Docker Hub installieren und ausführen können, ohne das Git-Repository zu klonen.

## Voraussetzungen

- Docker (und Docker Compose für die Compose-Methode)
- Für GPU-Versionen:
  - **NVIDIA GPU**: NVIDIA Docker Runtime ([Installationsanleitung](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html))
  - **AMD GPU**: ROCm-Unterstützung für Docker ([Installationsanleitung](https://rocm.docs.amd.com/en/latest/deploy/docker.html))
  - **Apple Silicon**: Keine speziellen Voraussetzungen, nur Docker Desktop für Apple Silicon

## Plattformspezifische Container

Double-Take Go Reborn bietet verschiedene Container-Varianten für unterschiedliche Hardware-Plattformen:

| Plattform | Docker-Tag | Beschreibung | Anforderungen |
|-----------|------------|--------------|---------------|
| CPU | `cpu` oder `latest` | Standard x86_64 Prozessoren | Mindestens 2 CPU-Kerne empfohlen |
| NVIDIA GPU | `nvidia` | CUDA-Beschleunigung | NVIDIA GPU mit CUDA-Unterstützung |
| AMD GPU | `amd` | OpenCL-Beschleunigung | AMD GPU mit ROCm/OpenCL-Unterstützung |
| Apple Silicon | `arm64` | Metal-Optimierung für M1/M2/M3 | Apple Silicon Prozessor (M1/M2/M3) |

## Installation

### Methode 1: Docker Compose (empfohlen)

1. **Erstellen Sie ein Verzeichnis für Double-Take**

```bash
mkdir -p ~/double-take
cd ~/double-take
mkdir -p config data
```

2. **Erstellen Sie eine `docker-compose.yml` Datei**

Wählen Sie eine der folgenden Konfigurationen je nach Ihrer Hardware:

**CPU-Version (Standard)**

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:cpu
    restart: unless-stopped
    volumes:
      - ./config:/config
      - ./data:/data
    ports:
      - "3000:3000"
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
```

**NVIDIA GPU-Version**

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:nvidia
    restart: unless-stopped
    volumes:
      - ./config:/config
      - ./data:/data
    ports:
      - "3000:3000"
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

**AMD GPU-Version**

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:amd
    restart: unless-stopped
    volumes:
      - ./config:/config
      - ./data:/data
    ports:
      - "3000:3000"
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
    devices:
      - /dev/kfd:/dev/kfd
      - /dev/dri:/dev/dri
```

**Apple Silicon-Version**

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:arm64
    restart: unless-stopped
    volumes:
      - ./config:/config
      - ./data:/data
    ports:
      - "3000:3000"
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
    platform: linux/arm64
```

3. **Erstellen Sie eine Basiskonfiguration**

Erstellen Sie eine Datei `config/config.yaml` mit folgendem Inhalt:

```yaml
server:
  port: 3000

database:
  path: "/data/double-take.db"

images:
  storage_path: "/data/images"

opencv:
  use_gpu: false  # Auf 'true' setzen für GPU-Versionen
  person_detection:
    enabled: true
    backend: "hog"  # Für NVIDIA: 'cuda', für AMD: 'opencl', für Apple Silicon: 'metal'
    confidence_threshold: 0.6

processor:
  max_workers: 2  # An Ihre CPU-Kernanzahl anpassen

frigate:
  enabled: false
  api_key: ""
  host: "http://frigate:5000"

compreface:
  enabled: false
  host: "http://compreface:8000"
  recognition_api_key: ""
  sync_interval_minutes: 15

mqtt:
  enabled: false
  host: "mqtt-broker"
  port: 1883
  topic: "frigate/events"
  client_id: "double-take"

hass:
  enabled: false
  host: "http://homeassistant:8123"
  token: ""
```

4. **Starten Sie den Container**

```bash
docker compose up -d
```

5. **Greifen Sie auf die Web-UI zu**

Öffnen Sie einen Browser und navigieren Sie zu:
```
http://localhost:3000
```

### Methode 2: Docker CLI

Wenn Sie Docker Compose nicht verwenden möchten, können Sie den Container auch direkt mit Docker starten:

**CPU-Version**

```bash
mkdir -p ~/double-take/config ~/double-take/data
cd ~/double-take

# Konfigurationsdatei erstellen (siehe oben)
# ...

docker run -d --name double-take \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/Berlin \
  silentbob69/double-take-go-reborn:cpu
```

**NVIDIA GPU-Version**

```bash
docker run -d --name double-take \
  --gpus all \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/Berlin \
  silentbob69/double-take-go-reborn:nvidia
```

**AMD GPU-Version**

```bash
docker run -d --name double-take \
  --device=/dev/kfd \
  --device=/dev/dri \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/Berlin \
  silentbob69/double-take-go-reborn:amd
```

**Apple Silicon-Version**

```bash
docker run -d --name double-take \
  --platform linux/arm64 \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/Berlin \
  silentbob69/double-take-go-reborn:arm64
```

## Konfiguration anpassen

Nachdem der Container läuft, können Sie die Konfiguration anpassen:

1. Bearbeiten Sie die Datei `config/config.yaml` mit einem Texteditor
2. Starten Sie den Container neu, damit die Änderungen wirksam werden:

```bash
docker compose restart double-take
# Oder bei Verwendung des Docker CLI:
docker restart double-take
```

## Updates

Um auf eine neuere Version zu aktualisieren:

```bash
# Mit Docker Compose:
docker compose pull
docker compose down
docker compose up -d

# Mit Docker CLI:
docker pull silentbob69/double-take-go-reborn:cpu  # oder nvidia, amd, arm64
docker stop double-take
docker rm double-take
# Container neu starten mit dem ursprünglichen Run-Befehl
```

## Fehlerbehebung

### Container startet nicht

Überprüfen Sie die Logs:

```bash
# Mit Docker Compose:
docker compose logs double-take

# Mit Docker CLI:
docker logs double-take
```

### GPU wird nicht erkannt

- **NVIDIA**: Überprüfen Sie mit `nvidia-smi`, ob die GPU korrekt erkannt wird
- **AMD**: Überprüfen Sie mit `rocm-smi`, ob die GPU korrekt erkannt wird
- Setzen Sie in der `config.yaml` die Werte `opencv.use_gpu: true` und den korrekten Backend-Wert

### Datenpersistenz

Alle Daten werden in den gemounteten Volumes gespeichert:
- `/config`: Konfigurationsdateien
- `/data`: Datenbank, Bilder und andere persistente Daten

Es wird empfohlen, regelmäßig Backups dieser Verzeichnisse anzulegen.
