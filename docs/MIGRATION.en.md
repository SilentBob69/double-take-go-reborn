# Migration Guide: Configuration Changes

This guide helps you migrate your existing Double-Take Go Reborn installation to the new configuration structure.

## What has changed?

In the latest version, the structure of configuration files has been improved:

1. **New directory structure:**
   - Active configurations: `/config/hardware/`
   - Example configurations: `/config/examples/platforms/`

2. **New file names:**
   - Old: `config.opencv-nvidia.yaml` → New: `config-nvidia-gpu.yaml`
   - Old: `config.opencv-amd.yaml` → New: `config-amd-gpu.yaml`
   - Old: `config.opencv-apple-silicon.yaml` → New: `config-apple-silicon.yaml`
   - Old: `config.opencv-cpu.yaml` → New: `config-cpu.yaml`

3. **Sensitive data protection:**
   - API keys in example configurations are replaced with placeholders
   - The `.gitignore` has been updated to exclude active configurations

## Migration Steps

### 1. Create a backup

Backup your current configuration:

```bash
# In the project directory
cp config/config.yaml config/config.backup.yaml

# If you have platform-specific configurations
mkdir -p backup/config
cp config/config.opencv-*.yaml backup/config/ 2>/dev/null || true
```

### 2. Update repositories

Update your local repository:

```bash
git pull origin main
```

### 3. Create new configurations

Create the directory for your active configurations:

```bash
mkdir -p config/hardware
```

### 4. Migrate configuration files

Choose the option appropriate for your platform:

#### For NVIDIA GPU systems

```bash
# Copy the example configuration
cp config/examples/platforms/config-nvidia-gpu.example.yaml config/hardware/config-nvidia-gpu.yaml

# Transfer your settings from the old configuration
# Important: API keys and sensitive data must be transferred manually
```

#### For AMD GPU systems

```bash
# Copy the example configuration
cp config/examples/platforms/config-amd-gpu.example.yaml config/hardware/config-amd-gpu.yaml

# Transfer your settings from the old configuration
# Important: API keys and sensitive data must be transferred manually
```

#### For Apple Silicon systems

```bash
# Copy the example configuration
cp config/examples/platforms/config-apple-silicon.example.yaml config/hardware/config-apple-silicon.yaml

# Transfer your settings from the old configuration
# Important: API keys and sensitive data must be transferred manually
```

#### For CPU systems

```bash
# Copy the example configuration
cp config/examples/platforms/config-cpu.example.yaml config/hardware/config-cpu.yaml

# Transfer your settings from the old configuration
# Important: API keys and sensitive data must be transferred manually
```

### 5. Update main configuration

Connect your main configuration to the platform-specific configuration:

```bash
# Ensure that the main configuration uses the correct platform-specific configuration
cp config/hardware/config-nvidia-gpu.yaml config/config.yaml  # Replace with your platform
```

### 6. Check important settings

Check the following important settings in your new configuration:

1. **CompreFace API key**: Ensure that the correct API key is in the configuration
2. **GPU configuration**: Verify the OpenCV settings for your hardware
3. **Network settings**: Make sure the correct server addresses and ports are configured
4. **Integration points**: Check MQTT, Home Assistant and other integration settings

### 7. Restart the system

After migration, restart the system:

```bash
# In the directory with docker-compose.yml
docker-compose down
docker-compose up -d
```

### 8. Verification and cleanup

If everything works, you can remove the old configuration files:

```bash
# Only after successful migration and verification
rm config/config.backup.yaml
rm -rf backup
```

## Troubleshooting

If you encounter problems during migration:

1. **Configuration errors**: Check the logs with `docker-compose logs`
2. **GPU detection**: Check on the diagnostics page if the GPU is correctly detected
3. **Back to backup**: In case of serious problems, you can return to your backup configuration

## Support

If you need further assistance, visit the [Discussions section](https://github.com/SilentBob69/double-take-go-reborn/discussions) on GitHub.
