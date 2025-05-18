# Double-Take Go

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24-blue.svg)](https://golang.org)
[![Status: Alpha](https://img.shields.io/badge/Status-Alpha-red.svg)]()
[![Docker](https://img.shields.io/badge/Docker-Required-blue.svg)]()

*Diese Seite auf [Deutsch](README.md) lesen*

A Go implementation inspired by [Double Take](https://github.com/jakowenko/double-take), a facial recognition and tracking system for smart homes and surveillance cameras.

> **Note**: This project is still in early development. It is functional but may have bugs and incomplete features. Contributions and feedback are welcome!

## Acknowledgement

This project is a reimplementation in Go and was heavily inspired by the excellent [Double Take](https://github.com/jakowenko/double-take) by [Jacob Kowenko](https://github.com/jakowenko). The original version uses Node.js and offers a broader range of features. This project aims to implement similar functionality in Go but has not yet achieved full feature parity.

If you're looking for a proven and complete solution, we recommend using the original project.

## Features

- Integration with CompreFace for facial recognition
  - Periodic synchronization of CompreFace subjects (every 15 minutes)
  - Automatic updating of local database with CompreFace data
- OpenCV integration for efficient person detection
  - Pre-filtering of images to reduce unnecessary API calls to CompreFace
  - GPU acceleration on supported hardware (NVIDIA, AMD, Apple Silicon)
  - Configurable parameters for optimal detection performance
- MQTT integration for receiving events from Frigate NVR
- Home Assistant integration via MQTT for automatic device discovery and status updates
- Real-time notifications via Server-Sent Events (SSE)
- Modern toast notifications for system events and user actions
- Web-based user interface for managing images and faces
- Automatic cleanup of older data
- RESTful API for integrations with other systems
- Detailed diagnostics page with system and database statistics

## Requirements

- Docker and Docker Compose for easy installation
- CompreFace instance (accessible as an external service at the URL specified in the configuration)
- Optional: MQTT broker (as an external service for integration with Frigate and Home Assistant)
- Optional: Frigate NVR (as an external service to provide camera events)
- Optional: Home Assistant (for automatic integration of recognition results)

## Quick Start

1. Clone the repository:
   ```bash
   git clone https://github.com/SilentBob69/double-take-go-reborn.git
   cd double-take-go-reborn
   ```

2. Run the initial setup to create personal hardware configurations:
   ```bash
   ./scripts/switch-config.sh --setup
   ```

3. Choose the appropriate hardware configuration:
   ```bash
   # Choose one of: cpu, nvidia, amd, apple
   ./scripts/switch-config.sh nvidia
   ```
   
   The script will automatically ask if you want to restart the container
   and will execute all necessary Docker commands if you confirm.

5. The application is now accessible at:
   - Double-Take UI: http://localhost:3000

## Docker Hub

Double-Take-Go-Reborn is also available as a ready-to-use Docker image on Docker Hub:

```bash
# Standard (latest)
docker pull silentbob69/double-take-go-reborn

# With explicit latest tag
docker pull silentbob69/double-take-go-reborn:latest
```

### Docker Configuration

The application can be started with the image from Docker Hub as follows:

```yaml
# docker-compose.yml example with Docker Hub image
services:
  double-take:
    image: silentbob69/double-take-go-reborn:latest
    restart: unless-stopped
    volumes:
      - ./config:/config  # Configuration files
      - ./data:/data      # Persistent data and snapshots
    ports:
      - "3000:3000"       # Web UI port
    environment:
      - TZ=Europe/Berlin  # Adjust timezone
```

### Volumes and Ports

The Docker image uses the following volumes and ports:

- **Volumes**:
  - `/config`: Contains the configuration files (`config.yaml`)
  - `/data`: Storage location for the database and snapshot images
  
- **Ports**:
  - `3000`: Web user interface

## Development Environment

For development, we provide a special Docker environment:

1. Start the development environment:
   ```bash
   docker-compose -f docker-compose.dev.yml up -d
   ```

2. Enter the container:
   ```bash
   docker exec -it double-take-go-reborn-go-dev-1 /bin/bash
   ```

3. Build the application in the container:
   ```bash
   go build -o /app/bin/double-take /app/cmd/server/main.go
   ```

4. Start the application in the container:
   ```bash
   /app/bin/double-take /app/config/config.yaml
   ```

5. Or use the helper scripts:
   ```bash
   ./build.sh dev  # Starts the development environment
   ./build.sh run  # Starts the production environment
   ```

## Configuration

The main configuration file is `config.yaml`. Important settings include:

- `server`: Hostnames and ports for the server
- `compreface`: Connection details for the external CompreFace instance
  - `sync_interval_minutes`: Interval in minutes for periodic CompreFace synchronization (default: 15)
- `mqtt`: MQTT broker configuration for Frigate integration
  - `homeassistant`: Settings for Home Assistant integration
- `frigate`: Configuration for connecting to Frigate NVR
- `cleanup`: Settings for automatic data deletion

### Example Configuration

```yaml
# config.yaml example
server:
  host: "0.0.0.0"           # Bind to all interfaces
  port: 3000                # Web UI port
  snapshot_dir: "/data/snapshots"  # Where snapshots are stored
  snapshot_url: "/snapshots"      # URL path for snapshots

log:
  level: "info"             # Log level (debug, info, warn, error)
  file: "/data/logs/double-take.log"  # Log file

db:
  file: "/data/double-take.db"  # SQLite database path

compreface:
  enabled: true
  url: "http://compreface-api:8000"  # URL to CompreFace API
  recognition_api_key: "your_recognition_api_key"  # API key generated by CompreFace
  detection_api_key: "your_detection_api_key"      # API key generated by CompreFace
  det_prob_threshold: 0.8    # Detection threshold (0.0-1.0)
  sync_interval_minutes: 15   # Synchronization interval

mqtt:
  enabled: true              # Enable/disable MQTT
  broker: "mosquitto"        # MQTT broker hostname/IP
  port: 1883                 # MQTT broker port
  username: ""               # Optional: MQTT username
  password: ""               # Optional: MQTT password
  client_id: "double-take-go"  # Client ID for MQTT
  topic: "frigate/events"    # Topic for Frigate events

frigate:
  api_url: "http://frigate:5000"  # Frigate API URL
  url: "http://frigate:5000"      # Frigate Web UI URL

cleanup:
  retention_days: 30         # Retention period for images in days
```

Copy the example configuration to a file named `config.yaml` in the `/config` directory and adjust it to your needs.

## New Features and Improvements

- **Periodic CompreFace Synchronization**: The application now automatically synchronizes data between CompreFace and the local database.
- **Toast Notifications**: Modern, non-blocking notifications for system events and user actions.
- **Improved Diagnostics Page**: Displays detailed information about the system, database, and CompreFace integration.
- **Image Reprocessing**: Images can now be reprocessed directly from the user interface.

## API Documentation

Double-Take Go provides a comprehensive REST API that allows other applications to interact with the system. A complete documentation of the API endpoints can be found here:

- [API Documentation (English)](docs/API.en.md)
- [API-Dokumentation (Deutsch)](docs/API.md)
- [OpenCV Integration](docs/opencv-integration.md)
- [Hardware Platforms](docs/PLATFORMS.en.md)
- [Container Management](docs/CONTAINER.en.md)
- [Migration](docs/MIGRATION.en.md)
- [API Documentation](docs/API.en.md)

The API enables control of all key functions of the system, including image processing, identity management, and system functions.

## Feedback Welcome!

We appreciate your interest in Double-Take Go Reborn and warmly invite you to provide feedback, ask questions, or submit improvement suggestions. Your contributions help us continuously improve the project!

- **Issues**: Found a bug or have an idea for a new feature? [Create an issue](https://github.com/SilentBob69/double-take-go-reborn/issues/new)!
- **Discussions**: Questions about usage or general discussions take place in the [Discussions section](https://github.com/SilentBob69/double-take-go-reborn/discussions).
- **Pull Requests**: Code contributions are very welcome! Check out our [contribution guidelines](CONTRIBUTING.md).

Any feedback is valuable, regardless of whether you're an experienced developer or just want to try out the project.

## Support

If you like this project and want to support its development:

- **PayPal**: [Buy me a beer](https://www.paypal.com/donate/?hosted_button_id=6FTKYDXJ7R7ZL) via PayPal as a thank you.

Any support, whether financial or through contributions to the project, is greatly appreciated and helps to further develop and improve Double-Take Go Reborn.

## OpenCV Integration for Person Detection

Double-Take-Go-Reborn integrates OpenCV for person detection to improve the efficiency and accuracy of facial recognition by only forwarding relevant images to CompreFace.

### Platform Support

Depending on your hardware, different optimized Docker images are provided:

- **CPU Version**: Works on all platforms, lowest system requirements
- **NVIDIA GPU Version**: Optimized performance through CUDA acceleration for NVIDIA graphics cards
- **AMD GPU Version**: OpenCL-accelerated variant for AMD graphics cards
- **Apple Silicon Version**: Specially optimized for M1/M2/M3 processors

### Switching Hardware Platforms

Double-Take Go Reborn includes a convenient script to switch between hardware platforms:

```bash
# First time setup (creates personal hardware configurations)
./scripts/switch-config.sh --setup

# List available configurations
./scripts/switch-config.sh --list

# Switch to a specific hardware platform
./scripts/switch-config.sh nvidia   # For NVIDIA GPU acceleration
./scripts/switch-config.sh amd      # For AMD GPU acceleration
./scripts/switch-config.sh cpu      # For CPU-only processing
./scripts/switch-config.sh apple    # For Apple Silicon

# Check current configuration
./scripts/switch-config.sh --status
```

The script will:
1. Copy the appropriate configuration file
2. Ask if you want to restart the container
3. Automatically use the correct Docker directory
4. Optionally display container logs

Before using, make sure to customize your personal configuration files in `config/my-hardware/` with the appropriate API keys and connection details.

### Configuration Options

The main OpenCV settings in the configuration file:

```yaml
opencv:
  enabled: true              # Enable/disable OpenCV integration
  use_gpu: false             # Use GPU acceleration (if available)
  person_detection:          # Parameters for person detection
    method: "hog"           # "hog" (CPU) or "dnn" (GPU)
    confidence_threshold: 0.5 # Detection threshold
```

For more detailed documentation, see [OpenCV Integration](docs/opencv-integration.md).

## Future Plans

- Improving facial recognition accuracy
- Expanding Home Assistant integration
- Integration with additional NVR systems
- Mobile app integration
- Extending API functionality
