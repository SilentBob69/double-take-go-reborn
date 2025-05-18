# Docker-Tags für Double-Take-Go-Reborn

Dieses Dokument beschreibt die verfügbaren Docker-Tags für Double-Take-Go-Reborn auf Docker Hub.

## Haupt-Tags

| Tag | Beschreibung | Basis-Plattform |
|-----|--------------|-----------------|
| `latest` | Aktuelle stabile Version (CPU-Version) | Debian Bullseye |
| `cpu` | Aktuelle CPU-Version | Debian Bullseye |
| `nvidia` | Aktuelle NVIDIA GPU-Version | NVIDIA CUDA 11.8.0 |
| `amd` | Aktuelle AMD GPU-Version | Ubuntu 22.04 mit OpenCL |
| `arm64` | Aktuelle ARM64/Apple Silicon-Version | Ubuntu 22.04 (arm64) |

## Versions-Tags

Zusätzlich zu den Haupt-Tags stellen wir für jede Release-Version spezifische Tags bereit:

| Tag-Format | Beschreibung | Beispiel |
|------------|--------------|----------|
| `cpu-[version]` | Spezifische CPU-Version | `cpu-1.0.0` |
| `nvidia-[version]` | Spezifische NVIDIA-Version | `nvidia-1.0.0` |
| `amd-[version]` | Spezifische AMD-Version | `amd-1.0.0` |
| `arm64-[version]` | Spezifische ARM64-Version | `arm64-1.0.0` |

## Verwendung

### Mit Docker Compose

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:latest  # oder nvidia, amd, arm64
    restart: unless-stopped
    volumes:
      - ./config:/config  # Konfigurationsdateien
      - ./data:/data      # Persistente Daten
    ports:
      - "3000:3000"       # Web UI
    environment:
      - TZ=Europe/Berlin  # Zeitzone anpassen
```

### Mit Docker CLI

```bash
# Standard CPU-Version
docker run -d --name double-take \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  silentbob69/double-take-go-reborn:latest

# NVIDIA GPU-Version (mit GPU-Zugriff)
docker run -d --name double-take \
  --gpus all \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  silentbob69/double-take-go-reborn:nvidia
```

## Hardware-Anforderungen

Jede Plattform hat unterschiedliche Hardware-Anforderungen:

* **CPU**: Funktioniert auf allen Systemen, minimal 2 CPU-Kerne empfohlen.
* **NVIDIA**: Erfordert eine NVIDIA GPU mit CUDA-Unterstützung und nvidia-docker.
* **AMD**: Erfordert eine AMD GPU mit ROCm/OpenCL-Unterstützung.
* **ARM64**: Optimiert für Apple Silicon (M1/M2/M3), funktioniert auch auf anderen ARM64-Prozessoren.
