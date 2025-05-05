# double-take-go-reborn

[![Status](https://img.shields.io/badge/status-early_development-orange.svg)](https://shields.io/)
[Lesen Sie dies auf Deutsch](README.de.md)

**Important Note:** This project is in a very early stage of development. Features might be incomplete, and errors may occur.

## Description

`double-take-go-reborn` is an application written in Go for processing events (e.g., images from cameras) received via MQTT. It integrates with the face recognition service [CompreFace](https://github.com/exadel-inc/CompreFace) to detect and identify people in the images. The application provides a web API and potentially a user interface for managing and displaying the results.

This project is a redevelopment ("Reborn") of a similar concept application, written in Go.

## Features (planned/partially implemented)

*   Receiving events via MQTT.
*   Integration with CompreFace for face/person recognition.
*   Storage of events, matches, and snapshots in an SQLite database.
*   Configuration via a YAML file (`config/config.yaml`).
*   Web API (and potentially UI) for retrieving data and managing identities.
*   Periodic synchronization of identities with CompreFace.
*   Automated cleanup of old data and snapshots.
*   Docker support for easy deployment.

## Technology Stack

*   **Language:** Go
*   **Web Framework:** Gin
*   **Database:** SQLite (via GORM)
*   **MQTT Client:** Paho MQTT Go Client
*   **Configuration:** Koanf
*   **Containerization:** Docker, Docker Compose
*   **Face Recognition (External):** CompreFace

## Getting Started

### Prerequisites

*   Docker and Docker Compose must be installed.
*   A running instance of CompreFace (either local or remote).
*   An MQTT broker.

### Configuration

1.  Copy or rename the example configuration `config/config.example.yaml` to `/config/config.yaml` (or mount your own configuration file to this path in the container).
2.  Adjust `config/config.yaml` to your environment, especially the connection details for MQTT and CompreFace, as well as directory paths.

### Running with Docker Compose

The easiest way to start the application is using the provided `docker-compose.yml` file:

```bash
docker-compose up -d
```

This starts the `double-take-go-reborn` service. Ensure that your MQTT broker and CompreFace are also running and accessible.

## Status

As mentioned earlier, the project is in **early development**. Changes to the API and functionality are likely.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
