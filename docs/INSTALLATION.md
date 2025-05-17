# Installation von Double-Take Go Reborn

Diese Anleitung führt Sie durch die Installation und Konfiguration von Double-Take Go Reborn auf verschiedenen Hardwareplattformen.

## Voraussetzungen

- Docker und Docker Compose
- Optional: CompreFace-Instanz (für Gesichtserkennung)
- Optional: MQTT-Broker (für Frigate NVR und Home Assistant)
- Optional: Frigate NVR (für Kamera-Events)
- Optional: Home Assistant (für automatische Integration)

## Schnellinstallation

1. Repository klonen:
   ```bash
   git clone https://github.com/SilentBob69/double-take-go-reborn.git
   cd double-take-go-reborn
   ```

2. Basierend auf Ihrer Hardware, wechseln Sie in das entsprechende Docker-Verzeichnis:

   ```bash
   # Standard-CPU-Version (für alle Plattformen)
   cd docker/cpu
   
   # ODER: NVIDIA GPU-Version (für CUDA-fähige NVIDIA-GPUs)
   cd docker/nvidia
   
   # ODER: AMD GPU-Version (für OpenCL-fähige AMD-GPUs)
   cd docker/amd
   
   # ODER: Apple Silicon-Version (für M1/M2/M3 Macs)
   cd docker/apple-silicon
   ```

3. Konfigurationsdatei erstellen:
   ```bash
   # Eine der Beispiel-Konfigurationen kopieren
   cp ../../config/examples/config.cpu.yaml ../../config/config.yaml
   
   # Konfigurationsdatei bearbeiten
   nano ../../config/config.yaml
   ```

4. Container bauen und starten:
   ```bash
   docker-compose up -d
   ```

5. Die Anwendung ist nun erreichbar unter:
   - Double-Take UI: http://localhost:3000

## Hardware-Spezifische Konfiguration

### CPU-Version (Standard)

Die CPU-Version funktioniert auf allen Plattformen und verwendet den HOG-Detektor für die Personenerkennung. Diese Version ist schnell, benötigt aber keine spezielle Hardware.

Wichtige Konfigurationsoptionen:
```yaml
opencv:
  enabled: true
  use_gpu: false
  person_detection:
    method: "hog"  # HOG-Detektor für CPU
```

### NVIDIA GPU-Version

Diese Version verwendet CUDA für beschleunigtes Deep Learning mit NVIDIA GPUs.

Voraussetzungen:
- NVIDIA Grafikkarte mit CUDA-Unterstützung
- NVIDIA Docker Runtime installiert

Wichtige Konfigurationsoptionen:
```yaml
opencv:
  enabled: true
  use_gpu: true
  person_detection:
    method: "dnn"
    backend: "cuda"
    target: "cuda"
```

### AMD GPU-Version

Diese Version verwendet OpenCL für beschleunigtes Deep Learning mit AMD GPUs.

Voraussetzungen:
- AMD Grafikkarte mit OpenCL-Unterstützung
- ROCm-Installation für Linux

Wichtige Konfigurationsoptionen:
```yaml
opencv:
  enabled: true
  use_gpu: true
  person_detection:
    method: "dnn"
    backend: "opencl"
    target: "opencl"
```

### Apple Silicon Version

Diese Version ist optimiert für Apple Silicon Prozessoren (M1/M2/M3).

Wichtige Konfigurationsoptionen:
```yaml
opencv:
  enabled: true
  use_gpu: true  # Verwendet Metal-Framework
  person_detection:
    method: "dnn"
```

## CompreFace-Integration

Double-Take Go Reborn kann mit einer CompreFace-Instanz für die Gesichtserkennung integriert werden. Um CompreFace zu aktivieren:

1. Entkommentieren Sie die CompreFace-Services in der docker-compose.yml:
   ```yaml
   compreface-postgres:
     image: postgres:13.4
     # weitere Konfiguration...
   
   compreface-api:
     image: exadel/compreface-api:latest
     # weitere Konfiguration...
   
   compreface-ui:
     image: exadel/compreface-fe:latest
     # weitere Konfiguration...
   ```

2. Passen Sie die CompreFace-Konfiguration in der config.yaml an:
   ```yaml
   compreface:
     enabled: true
     url: "http://compreface-api:8000"
     recognition_api_key: "Ihr_API_Key"  # Von CompreFace generierter API-Key
   ```

## Frigate NVR und MQTT-Integration

Für die Integration mit Frigate NVR:

1. Stellen Sie sicher, dass Sie einen MQTT-Broker haben (z.B. Mosquitto)

2. Konfigurieren Sie die MQTT-Integration in der config.yaml:
   ```yaml
   mqtt:
     enabled: true
     broker: "192.168.1.100"  # IP-Adresse Ihres MQTT-Brokers
     port: 1883
     username: "mqtt_user"    # Optional
     password: "mqtt_pass"    # Optional
     topic: "frigate/events"  # Topic für Frigate-Events
   ```

3. Optional: Konfigurieren Sie die Frigate-Integration:
   ```yaml
   frigate:
     enabled: true
     api_url: "http://192.168.1.100:5000"  # Frigate API-URL
   ```

## Fehlersuche

Falls Probleme auftreten:

1. Container-Logs prüfen:
   ```bash
   docker-compose logs -f
   ```

2. Überprüfen Sie, ob alle erforderlichen Ports erreichbar sind

3. Stellen Sie sicher, dass die Konfigurationsdatei korrekt eingerichtet ist

4. Bei GPU-Versionen: Überprüfen Sie, ob die entsprechenden Treiber installiert sind

Weitere Hilfe finden Sie in der [Fehlersuche-Dokumentation](TROUBLESHOOTING.md) oder im [Issues-Bereich auf GitHub](https://github.com/SilentBob69/double-take-go-reborn/issues).
