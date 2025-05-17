# Hardware Platforms for Double-Take Go Reborn

This documentation describes the different supported hardware platforms and their configuration for Double-Take Go Reborn.

## Supported Hardware Platforms

Double-Take Go Reborn is optimized for various hardware architectures and offers the best possible performance on each platform through customized OpenCV configuration.

### CPU (Intel/AMD)

**Dockerfile**: `docker/cpu/Dockerfile`  
**Docker Compose**: `docker/cpu/docker-compose.yml`

The CPU version is the most universal configuration and works on all standard x86_64 processors without special hardware requirements.

#### Prerequisites
- Docker installed
- Minimum 2GB RAM, 4GB recommended
- x86_64 CPU (Intel or AMD)

#### Starting
```bash
cd docker/cpu
docker compose up -d
cp ../config/examples/platforms/config-cpu.example.yaml ../config/config.yaml
# Edit the file to update API keys and other sensitive data
```

#### OpenCV Configuration
The CPU version is built with standard OpenCV modules and uses CPU-based algorithms for image processing. All necessary modules like ArUco (for internal functions) are enabled.

### NVIDIA GPU

**Dockerfile**: `docker/nvidia/Dockerfile`  
**Docker Compose**: `docker/nvidia/docker-compose.yml`

This version is optimized for systems with NVIDIA graphics cards and uses CUDA for accelerated image processing.

#### Prerequisites
- Docker with NVIDIA support installed
- NVIDIA drivers installed
- NVIDIA GPU with CUDA support

#### Installation
1. Make sure Docker is configured with NVIDIA GPU support:
   ```bash
   # For newer Docker versions with nvidia-container-toolkit
   sudo apt-get install -y nvidia-container-toolkit
   sudo systemctl restart docker
   ```

2. Start the container and use the corresponding configuration:
   ```bash
   cd docker/nvidia
   docker compose up -d
   cp ../config/examples/platforms/config-nvidia-gpu.example.yaml ../config/config.yaml
   # Edit the file to update API keys and other sensitive data
   ```

#### OpenCV Configuration
The NVIDIA version is built with CUDA support and uses the GPU for accelerated image processing. Specific optimizations include:
- CUDA-enabled OpenCV modules
- CUDA support for DNN (Deep Neural Networks)
- NVIDIA Video Codec SDK for hardware-accelerated video decoding

### AMD GPU

**Dockerfile**: `docker/amd/Dockerfile`  
**Docker Compose**: `docker/amd/docker-compose.yml`

This version is optimized for systems with AMD graphics cards and uses OpenCL for accelerated image processing.

#### Prerequisites
- Docker installed
- AMD GPU with OpenCL support
- ROCm drivers installed

#### Installation
1. Make sure the ROCm drivers are installed:
   ```bash
   # Installation of ROCm drivers (Ubuntu example)
   sudo apt-get update
   sudo apt-get install -y rocm-dev
   ```

2. Start the container and use the corresponding configuration:
   ```bash
   cd docker/amd
   docker compose up -d
   cp ../config/examples/platforms/config-amd-gpu.example.yaml ../config/config.yaml
   # Edit the file to update API keys and other sensitive data
   ```

#### OpenCV Configuration
The AMD version is built with OpenCL support and uses the GPU for accelerated image processing. Specific optimizations include:
- OpenCL-enabled OpenCV modules
- OpenCL support for DNN

### Apple Silicon (M1/M2/M3)

**Dockerfile**: `docker/apple-silicon/Dockerfile`  
**Docker Compose**: `docker/apple-silicon/docker-compose.yml`

This version is specifically optimized for Apple Silicon (ARM64) processors like M1, M2, and M3.

#### Prerequisites
- macOS on Apple Silicon (M1/M2/M3)
- Docker Desktop for Apple Silicon installed

#### Starting
```bash
cd docker/apple-silicon
docker compose up -d
cp ../config/examples/platforms/config-apple-silicon.example.yaml ../config/config.yaml
# Edit the file to update API keys and other sensitive data
```

#### OpenCV Configuration
The Apple Silicon version is built with ARM-specific optimizations:
- NEON vectorization for improved performance
- Optimizations for ARM64 architecture

## Known Issues and Solutions

### ArUco Module Dependencies

The GoCV library requires the ArUco module from OpenCV, even if it is not directly used by the application. All Docker configurations are set to correctly build this module.

If you create your own Docker builds, make sure not to disable the ArUco module, otherwise you may encounter build errors.

### Platform-Specific Builds

The Docker configurations are platform-specifically optimized. Do not try to use an image built for one platform on another (e.g., an NVIDIA image on a system without an NVIDIA GPU).

## Performance Tips

- **CPU Version**: The CPU version can optimally utilize multi-core processors. The number of available cores can be adjusted in the configuration.
- **GPU Versions**: The GPU versions benefit from modern graphics cards with plenty of VRAM. If VRAM is limited, the resolution of the images can be reduced in the configuration.
- **Apple Silicon**: The Apple Silicon version uses specially optimized ARM vector instructions and offers the best performance on compatible Macs.

## Advanced Configuration

For detailed information on further configuration of the individual platforms, see the [Configuration Documentation](CONFIGURATION.en.md).
