# Container Management for Double-Take Go Reborn

This documentation describes how to effectively manage, update, and diagnose containers for Double-Take Go Reborn.

## Container Updates

After updating the source code or configuration, containers need to be rebuilt to incorporate the changes.

### Standard Update Process

```bash
# 1. Change to the appropriate hardware directory (nvidia, amd, cpu, apple-silicon)
cd docker/nvidia  # As an example for NVIDIA GPU

# 2. Stop current containers
docker compose down

# 3. Rebuild images with --no-cache option for a complete rebuild
docker compose build --no-cache

# 4. Restart containers
docker compose up -d

# 5. Check logs for potential errors
docker compose logs -f
```

### Incremental Update (faster)

If only minor changes were made and a full rebuild is not necessary:

```bash
# 1. Stop containers
docker compose down

# 2. Build images using existing cache
docker compose build

# 3. Restart containers
docker compose up -d
```

## Data Persistence During Updates

Double-Take Go Reborn stores data in volume mounts. These remain intact during container updates.

### Important Data Paths

- **/config**: Contains configuration files (stored on the host system)
- **/data**: Contains databases, processed images, snapshots, etc.

### Data Backup Before Updates

It is recommended to create a backup before major updates:

```bash
# 1. Create backup directory
mkdir -p ~/double-take-backup/$(date +%Y%m%d)

# 2. Backup configuration
cp -r /path/to/double-take-go-reborn/config ~/double-take-backup/$(date +%Y%m%d)/

# 3. Backup data (if available on the host)
cp -r /path/to/double-take-go-reborn/data ~/double-take-backup/$(date +%Y%m%d)/

# Alternative: Directly backup Docker volume
docker run --rm -v double-take-data:/data -v ~/double-take-backup/$(date +%Y%m%d):/backup alpine tar -czf /backup/data.tar.gz /data
```

## Container Diagnostics and Troubleshooting

### Checking Logs

```bash
# View container logs
docker compose logs -f
```

### Common Issues and Solutions

#### 1. Container Does Not Start

**Symptom**: The container exits immediately after starting or doesn't start at all.

**Solutions**:
- Check logs: `docker compose logs`
- Verify configuration file for errors
- For NVIDIA/AMD: Test GPU drivers and Docker integration: `nvidia-smi` or `rocm-smi`

#### 2. GPU Acceleration Issues

**Symptom**: Container starts, but OpenCV doesn't use GPU acceleration.

**Solutions**:
- Check if the container runtime is properly configured
- For NVIDIA: `sudo nvidia-ctk runtime configure --runtime=docker`
- Check config file: `opencv.use_gpu` and `opencv.person_detection.backend` must be set correctly

#### 3. OpenCV Dependency Problems

**Symptom**: Error messages regarding missing libraries or modules.

**Solutions**:
- Use only the official Docker configurations without modifying the Dockerfile
- For custom builds: Ensure all required OpenCV modules are enabled, especially ArUco

#### 4. Performance Issues

**Symptom**: High CPU usage or slow processing.

**Solutions**:
- Check hardware-specific configuration
- For GPU: Verify if the correct GPU is detected and can be used
- Adjust worker pool size in the configuration (`processor.max_workers`)

## Manual Container Creation

If more specialized configurations are needed:

```bash
# Manual build with specific parameters
docker build -t double-take-go:custom -f docker/nvidia/Dockerfile \
  --build-arg BUILDPLATFORM=linux/amd64 \
  --build-arg TARGETPLATFORM=linux/amd64 \
  .

# Start container manually
docker run -d --name double-take \
  -p 3000:3000 \
  -v /path/to/config:/config \
  -v double-take-data:/data \
  --gpus all \  # Only for NVIDIA
  double-take-go:custom
```

## Managing Container Versions

It is recommended to tag container versions with either version numbers or commit hashes:

```bash
# Tag image with version
docker tag double-take-go:latest double-take-go:v1.2.3

# Roll back if there are issues
docker compose down
# Modify the docker-compose.yml or use an alternative compose file with the older tag
docker compose -f docker-compose.old.yml up -d
```
