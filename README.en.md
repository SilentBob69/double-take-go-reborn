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

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/SilentBob69/double-take-go-reborn.git
   cd double-take-go-reborn
   ```

2. Create the configuration file:
   ```bash
   cp config/config.example.yaml config/config.yaml
   ```

3. Adjust the configuration file (IP addresses, API keys, etc.):
   ```bash
   nano config/config.yaml
   ```

4. Start the application with Docker Compose:
   ```bash
   docker-compose up -d
   ```

5. The application is now accessible at:
   - Double-Take UI: http://localhost:3000

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

## New Features and Improvements

- **Periodic CompreFace Synchronization**: The application now automatically synchronizes data between CompreFace and the local database.
- **Toast Notifications**: Modern, non-blocking notifications for system events and user actions.
- **Improved Diagnostics Page**: Displays detailed information about the system, database, and CompreFace integration.
- **Image Reprocessing**: Images can now be reprocessed directly from the user interface.

## Future Plans

- Improving facial recognition accuracy
- Expanding Home Assistant integration
- Integration with additional NVR systems
- Mobile app integration
- API documentation with Swagger/OpenAPI
