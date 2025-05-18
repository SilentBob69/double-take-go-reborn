# Container-Management für Double-Take Go Reborn

Diese Dokumentation beschreibt, wie Container für Double-Take Go Reborn effektiv verwaltet, aktualisiert und bei Problemen diagnostiziert werden können.

## Container-Aktualisierung

Nach einem Update des Quellcodes oder der Konfiguration müssen die Container neu gebaut werden, um die Änderungen zu übernehmen.

### Standard-Update-Prozess

```bash
# 1. Zum entsprechenden Hardware-Verzeichnis wechseln (nvidia, amd, cpu, apple-silicon)
cd docker/nvidia  # Als Beispiel für NVIDIA-GPU

# 2. Aktuelle Container stoppen
docker compose down

# 3. Images neu bauen mit --no-cache Option für vollständigen Rebuild
docker compose build --no-cache

# 4. Container neu starten
docker compose up -d

# 5. Logs prüfen auf mögliche Fehler
docker compose logs -f
```

### Inkrementelles Update (schneller)

Wenn nur kleine Änderungen gemacht wurden und ein vollständiger Rebuild nicht nötig ist:

```bash
# 1. Container stoppen
docker compose down

# 2. Images bauen mit vorhandenem Cache
docker compose build

# 3. Container neu starten
docker compose up -d
```

## Datenpersistenz bei Updates

Double-Take Go Reborn speichert Daten in Volume-Mounts. Diese bleiben bei Container-Updates erhalten.

### Wichtige Datenpfade

- **/config**: Enthält die Konfigurationsdateien (wird auf dem Host-System gespeichert)
- **/data**: Enthält Datenbanken, verarbeitete Bilder, Snapshots, etc.

### Datensicherung vor Updates

Es wird empfohlen, vor größeren Updates eine Sicherung anzulegen:

```bash
# 1. Backup-Verzeichnis erstellen
mkdir -p ~/double-take-backup/$(date +%Y%m%d)

# 2. Konfiguration sichern
cp -r /pfad/zu/double-take-go-reborn/config ~/double-take-backup/$(date +%Y%m%d)/

# 3. Daten sichern (wenn auf dem Host verfügbar)
cp -r /pfad/zu/double-take-go-reborn/data ~/double-take-backup/$(date +%Y%m%d)/

# Alternativ: Docker Volume direkt sichern
docker run --rm -v double-take-data:/data -v ~/double-take-backup/$(date +%Y%m%d):/backup alpine tar -czf /backup/data.tar.gz /data
```

## Container-Diagnose und Problembehebung

### Logs überprüfen

```bash
# Container-Logs anzeigen
docker compose logs -f
```

### Häufige Probleme und Lösungen

#### 1. Container startet nicht

**Symptom**: Der Container beendet sich sofort nach dem Start oder startet gar nicht.

**Lösungen**:
- Logs überprüfen: `docker compose logs`
- Konfigurationsdatei auf Fehler prüfen
- Bei NVIDIA/AMD: GPU-Treiber und Docker-Integration testen: `nvidia-smi` oder `rocm-smi`

#### 2. Fehler mit GPU-Beschleunigung

**Symptom**: Container startet, aber OpenCV nutzt keine GPU-Beschleunigung.

**Lösungen**:
- Prüfen, ob die Container-Runtime richtig konfiguriert ist
- Für NVIDIA: `sudo nvidia-ctk runtime configure --runtime=docker`
- Config-Datei prüfen: `opencv.use_gpu` und `opencv.person_detection.backend` müssen korrekt gesetzt sein

#### 3. OpenCV Abhängigkeitsprobleme

**Symptom**: Fehlermeldungen bezüglich fehlender Bibliotheken oder Module.

**Lösungen**:
- Verwende nur die offiziellen Docker-Konfigurationen ohne Änderungen am Dockerfile
- Bei eigenen Builds: Stelle sicher, dass alle benötigten OpenCV-Module aktiviert sind, besonders ArUco

#### 4. Performance-Probleme

**Symptom**: Hohe CPU-Last oder langsame Verarbeitung.

**Lösungen**:
- Hardware-spezifische Konfiguration prüfen
- Bei GPU: Prüfen, ob die richtige GPU erkannt wird und genutzt werden kann
- Worker-Pool-Größe in der Konfiguration anpassen (`processor.max_workers`)

## Manuelle Container-Erstellung

Falls speziellere Konfigurationen benötigt werden:

```bash
# Manueller Build mit spezifischen Parametern
docker build -t double-take-go:custom -f docker/nvidia/Dockerfile \
  --build-arg BUILDPLATFORM=linux/amd64 \
  --build-arg TARGETPLATFORM=linux/amd64 \
  .

# Container manuell starten
docker run -d --name double-take \
  -p 3000:3000 \
  -v /pfad/zu/config:/config \
  -v double-take-data:/data \
  --gpus all \  # Nur für NVIDIA
  double-take-go:custom
```

## Container-Versionen verwalten

Es wird empfohlen, Container-Versionen mit Tags zu versehen, die der Versionsnummer oder dem Commit-Hash entsprechen:

```bash
# Image mit Version taggen
docker tag double-take-go:latest double-take-go:v1.2.3

# Bei Problemen zurückrollen
docker compose down
# Die docker-compose.yml anpassen oder ein alternatives Compose-File mit dem älteren Tag verwenden
docker compose -f docker-compose.old.yml up -d
```
