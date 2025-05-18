# Docker Tags for Double-Take-Go-Reborn

This document describes the available Docker tags for Double-Take-Go-Reborn on Docker Hub.

## Main Tags

| Tag | Description | Base Platform |
|-----|-------------|---------------|
| `latest` | Current stable version (CPU version) | Debian Bullseye |
| `cpu` | Current CPU version | Debian Bullseye |
| `nvidia` | Current NVIDIA GPU version | NVIDIA CUDA 11.8.0 |
| `amd` | Current AMD GPU version | Ubuntu 22.04 with OpenCL |
| `arm64` | Current ARM64/Apple Silicon version | Ubuntu 22.04 (arm64) |

## Version Tags

In addition to the main tags, we provide specific tags for each release version:

| Tag Format | Description | Example |
|------------|-------------|---------|
| `cpu-[version]` | Specific CPU version | `cpu-1.0.0` |
| `nvidia-[version]` | Specific NVIDIA version | `nvidia-1.0.0` |
| `amd-[version]` | Specific AMD version | `amd-1.0.0` |
| `arm64-[version]` | Specific ARM64 version | `arm64-1.0.0` |

## Usage

### With Docker Compose

```yaml
services:
  double-take:
    image: silentbob69/double-take-go-reborn:latest  # or nvidia, amd, arm64
    restart: unless-stopped
    volumes:
      - ./config:/config  # Configuration files
      - ./data:/data      # Persistent data
    ports:
      - "3000:3000"       # Web UI
    environment:
      - TZ=Europe/Berlin  # Adjust timezone
```

### With Docker CLI

```bash
# Standard CPU version
docker run -d --name double-take \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  silentbob69/double-take-go-reborn:latest

# NVIDIA GPU version (with GPU access)
docker run -d --name double-take \
  --gpus all \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  silentbob69/double-take-go-reborn:nvidia
```

## Hardware Requirements

Each platform has different hardware requirements:

* **CPU**: Works on all systems, minimum 2 CPU cores recommended.
* **NVIDIA**: Requires an NVIDIA GPU with CUDA support and nvidia-docker.
* **AMD**: Requires an AMD GPU with ROCm/OpenCL support.
* **ARM64**: Optimized for Apple Silicon (M1/M2/M3), also works on other ARM64 processors.
