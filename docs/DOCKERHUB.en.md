# Installation with Docker Hub

This guide describes how to install and run Double-Take Go Reborn directly from Docker Hub without cloning the Git repository.

## Prerequisites

- Docker (and Docker Compose for the compose method)
- For GPU versions:
  - **NVIDIA GPU**: NVIDIA Docker Runtime ([Installation Guide](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html))
  - **AMD GPU**: ROCm support for Docker ([Installation Guide](https://rocm.docs.amd.com/en/latest/deploy/docker.html))
  - **Apple Silicon**: No special prerequisites, just Docker Desktop for Apple Silicon

## Platform-Specific Containers

Double-Take Go Reborn offers different container variants for various hardware platforms:

| Platform | Docker Tag | Description | Requirements |
|-----------|------------|--------------|---------------|
| CPU | `cpu` or `latest` | Standard x86_64 processors | At least 2 CPU cores recommended |
| NVIDIA GPU | `nvidia` | CUDA acceleration | NVIDIA GPU with CUDA support |
| AMD GPU | `amd` | OpenCL acceleration | AMD GPU with ROCm/OpenCL support |
| Apple Silicon | `arm64` | Metal optimization for M1/M2/M3 | Apple Silicon processor (M1/M2/M3) |

## Installation

### Method 1: Docker Compose (recommended)

1. **Create a directory for Double-Take**

```bash
mkdir -p ~/double-take
cd ~/double-take
mkdir -p config data
```

2. **Create a `docker-compose.yml` file**

Choose one of the following configurations depending on your hardware:

**CPU version (standard)**

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
      - TZ=Europe/London  # Adjust timezone
```

**NVIDIA GPU version**

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
      - TZ=Europe/London  # Adjust timezone
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

**AMD GPU version**

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
      - TZ=Europe/London  # Adjust timezone
    devices:
      - /dev/kfd:/dev/kfd
      - /dev/dri:/dev/dri
```

**Apple Silicon version**

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
      - TZ=Europe/London  # Adjust timezone
    platform: linux/arm64
```

3. **Create a basic configuration**

Create a file `config/config.yaml` with the following content:

```yaml
server:
  port: 3000

database:
  path: "/data/double-take.db"

images:
  storage_path: "/data/images"

opencv:
  use_gpu: false  # Set to 'true' for GPU versions
  person_detection:
    enabled: true
    backend: "hog"  # For NVIDIA: 'cuda', for AMD: 'opencl', for Apple Silicon: 'metal'
    confidence_threshold: 0.6

processor:
  max_workers: 2  # Adjust according to your CPU core count

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

4. **Start the container**

```bash
docker compose up -d
```

5. **Access the web UI**

Open a browser and navigate to:
```
http://localhost:3000
```

### Method 2: Docker CLI

If you don't want to use Docker Compose, you can also start the container directly with Docker:

**CPU version**

```bash
mkdir -p ~/double-take/config ~/double-take/data
cd ~/double-take

# Create configuration file (see above)
# ...

docker run -d --name double-take \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/London \
  silentbob69/double-take-go-reborn:cpu
```

**NVIDIA GPU version**

```bash
docker run -d --name double-take \
  --gpus all \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/London \
  silentbob69/double-take-go-reborn:nvidia
```

**AMD GPU version**

```bash
docker run -d --name double-take \
  --device=/dev/kfd \
  --device=/dev/dri \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/London \
  silentbob69/double-take-go-reborn:amd
```

**Apple Silicon version**

```bash
docker run -d --name double-take \
  --platform linux/arm64 \
  -p 3000:3000 \
  -v $(pwd)/config:/config \
  -v $(pwd)/data:/data \
  -e TZ=Europe/London \
  silentbob69/double-take-go-reborn:arm64
```

## Adjust configuration

After the container is running, you can adjust the configuration:

1. Edit the file `config/config.yaml` with a text editor
2. Restart the container for the changes to take effect:

```bash
docker compose restart double-take
# Or when using Docker CLI:
docker restart double-take
```

## Updates

To upgrade to a newer version:

```bash
# With Docker Compose:
docker compose pull
docker compose down
docker compose up -d

# With Docker CLI:
docker pull silentbob69/double-take-go-reborn:cpu  # or nvidia, amd, arm64
docker stop double-take
docker rm double-take
# Restart the container with the original run command
```

## Troubleshooting

### Container doesn't start

Check the logs:

```bash
# With Docker Compose:
docker compose logs double-take

# With Docker CLI:
docker logs double-take
```

### GPU is not recognized

- **NVIDIA**: Check with `nvidia-smi` if the GPU is correctly recognized
- **AMD**: Check with `rocm-smi` if the GPU is correctly recognized
- Set in `config.yaml` the values `opencv.use_gpu: true` and the correct backend value

### Data persistence

All data is stored in the mounted volumes:
- `/config`: Configuration files
- `/data`: Database, images, and other persistent data

It is recommended to create regular backups of these directories.
