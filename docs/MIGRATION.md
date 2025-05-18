# Migrationsleitfaden: Konfigurationsänderungen

Dieser Leitfaden hilft dir, deine bestehende Double-Take Go Reborn Installation auf die neue Konfigurationsstruktur umzustellen.

## Was hat sich geändert?

In der neuesten Version wurde die Struktur der Konfigurationsdateien verbessert:

1. **Neue Verzeichnisstruktur:**
   - Aktive Konfigurationen: `/config/hardware/`
   - Beispielkonfigurationen: `/config/examples/platforms/`

2. **Neue Dateinamen:**
   - Alt: `config.opencv-nvidia.yaml` → Neu: `config-nvidia-gpu.yaml`
   - Alt: `config.opencv-amd.yaml` → Neu: `config-amd-gpu.yaml`
   - Alt: `config.opencv-apple-silicon.yaml` → Neu: `config-apple-silicon.yaml`
   - Alt: `config.opencv-cpu.yaml` → Neu: `config-cpu.yaml`

3. **Sensible Daten geschützt:**
   - API-Keys in Beispielkonfigurationen werden durch Platzhalter ersetzt
   - Die `.gitignore` wurde aktualisiert, um die aktiven Konfigurationen auszuschließen

## Migrationsschritte

### 1. Backup erstellen

Sichere deine aktuelle Konfiguration:

```bash
# Im Projektverzeichnis
cp config/config.yaml config/config.backup.yaml

# Falls du plattformspezifische Konfigurationen hast
mkdir -p backup/config
cp config/config.opencv-*.yaml backup/config/ 2>/dev/null || true
```

### 2. Repositories aktualisieren

Aktualisiere dein lokales Repository:

```bash
git pull origin main
```

### 3. Neue Konfigurationen erstellen

Erstelle das Verzeichnis für deine aktiven Konfigurationen:

```bash
mkdir -p config/hardware
```

### 4. Konfigurationsdateien migrieren

Wähle die für deine Plattform passende Option:

#### Für NVIDIA-GPU-Systeme

```bash
# Kopiere die Beispielkonfiguration
cp config/examples/platforms/config-nvidia-gpu.example.yaml config/hardware/config-nvidia-gpu.yaml

# Übertrage deine Einstellungen von der alten Konfiguration
# Wichtig: API-Keys und sensible Daten müssen manuell übertragen werden
```

#### Für AMD-GPU-Systeme

```bash
# Kopiere die Beispielkonfiguration
cp config/examples/platforms/config-amd-gpu.example.yaml config/hardware/config-amd-gpu.yaml

# Übertrage deine Einstellungen von der alten Konfiguration
# Wichtig: API-Keys und sensible Daten müssen manuell übertragen werden
```

#### Für Apple Silicon-Systeme

```bash
# Kopiere die Beispielkonfiguration
cp config/examples/platforms/config-apple-silicon.example.yaml config/hardware/config-apple-silicon.yaml

# Übertrage deine Einstellungen von der alten Konfiguration
# Wichtig: API-Keys und sensible Daten müssen manuell übertragen werden
```

#### Für CPU-Systeme

```bash
# Kopiere die Beispielkonfiguration
cp config/examples/platforms/config-cpu.example.yaml config/hardware/config-cpu.yaml

# Übertrage deine Einstellungen von der alten Konfiguration
# Wichtig: API-Keys und sensible Daten müssen manuell übertragen werden
```

### 5. Hauptkonfiguration aktualisieren

Verbinde deine Hauptkonfiguration mit der plattformspezifischen Konfiguration:

```bash
# Sicherstellen, dass die Hauptkonfiguration die richtige plattformspezifische Konfiguration verwendet
cp config/hardware/config-nvidia-gpu.yaml config/config.yaml  # Ersetze mit deiner Plattform
```

### 6. Wichtige Einstellungen prüfen

Überprüfe folgende wichtige Einstellungen in deiner neuen Konfiguration:

1. **CompreFace API-Key**: Stelle sicher, dass der korrekte API-Key in der Konfiguration steht
2. **GPU-Konfiguration**: Überprüfe die OpenCV-Einstellungen für deine Hardware
3. **Netzwerkeinstellungen**: Stelle sicher, dass die richtigen Serveradressen und Ports konfiguriert sind
4. **Integrationspunkte**: Überprüfe MQTT, Home Assistant und andere Integrationseinstellungen

### 7. Neustart des Systems

Nach der Migration starte das System neu:

```bash
# Im Verzeichnis mit der docker-compose.yml
docker-compose down
docker-compose up -d
```

### 8. Überprüfung und Bereinigung

Wenn alles funktioniert, kannst du die alten Konfigurationsdateien entfernen:

```bash
# Erst nach erfolgreicher Migration und Überprüfung
rm config/config.backup.yaml
rm -rf backup
```

## Problemlösungen

Solltest du Probleme bei der Migration haben:

1. **Konfigurationsfehler**: Überprüfe die Logs mit `docker-compose logs`
2. **GPU-Erkennung**: Prüfe auf der Diagnoseseite, ob die GPU korrekt erkannt wird
3. **Zurück zum Backup**: Bei schwerwiegenden Problemen kannst du zu deiner Backup-Konfiguration zurückkehren

## Support

Wenn du weitere Hilfe benötigst, besuche die [Discussions-Sektion](https://github.com/SilentBob69/double-take-go-reborn/discussions) auf GitHub.
