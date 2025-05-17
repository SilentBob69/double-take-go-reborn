# OpenCV Integration for Double-Take

This document describes how to use Double-Take with OpenCV integration for person detection.

## Overview

Double-Take supports OpenCV-based person detection as a pre-filter before face recognition with CompreFace. This can significantly improve performance by reducing unnecessary API calls to CompreFace.

## Supported Platforms

OpenCV integration works on multiple hardware platforms with optimized configurations:

### 1. CPU (Standard)
- Works on all hardware
- Uses HOG detector by default (faster but less accurate)
- Configuration: `config/config.opencv-cpu.yaml`
- Dockerfile: `Dockerfile.opencv`

### 2. NVIDIA GPU
- Requires NVIDIA GPU with CUDA support
- Uses DNN detector with CUDA acceleration
- Configuration: `config/config.opencv-nvidia.yaml`
- Dockerfile: `Dockerfile.opencv-cuda`

### 3. AMD GPU
- Requires AMD GPU with OpenCL support
- Uses DNN detector with OpenCL acceleration
- Configuration: `config/config.opencv-amd.yaml`
- Dockerfile: `Dockerfile.opencv-opencl`

### 4. Apple Silicon
- Optimized for M1/M2/M3 processors
- Uses DNN detector with Metal performance optimization
- Configuration: `config/config.opencv-apple-silicon.yaml`
- Dockerfile: `Dockerfile.opencv-arm64`

## Usage

1. Copy the appropriate config file to `config/config.yaml`
2. Use the corresponding Dockerfile to build your container:

```bash
# For CPU version:
docker-compose -f docker-compose.opencv.example.yml up -d double-take

# For NVIDIA GPU version:
docker-compose -f docker-compose.opencv.example.yml up -d double-take-cuda

# For AMD GPU version:
docker-compose -f docker-compose.opencv.example.yml up -d double-take-opencl

# For Apple Silicon:
docker-compose -f docker-compose.opencv.example.yml up -d double-take-arm64
```

## Configuration Options

The person detection can be configured with the following options:

```yaml
opencv:
  enabled: true
  use_gpu: false  # Set to true for GPU versions
  person_detection:
    method: "hog"  # Options: "hog" (for CPU), "dnn" (for GPU)
    confidence_threshold: 0.5
    scale_factor: 1.05
    min_neighbors: 2
    min_size_width: 64
    min_size_height: 128
    # GPU-specific parameters (only used when use_gpu is true and method is "dnn")
    backend: "default"  # Options: "default", "cuda" (NVIDIA), "opencl" (AMD)
    target: "cpu"       # Options: "cpu", "cuda", "opencl"
```

## Disabling OpenCV

If you don't need person detection, you can disable it by setting:

```yaml
opencv:
  enabled: false
```

This will skip the person detection step and directly use CompreFace for all images.
