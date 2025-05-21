#!/bin/bash
# improved-switch-config.sh - Verbesserte Version, die nur hardware-spezifische Konfigurationen ändert
# Autor: Double-Take Team
# Datum: 2025-05-21

set -e  # Bei Fehlern abbrechen

# Pfade automatisch ermitteln (funktioniert unabhängig vom Installationsort)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_ROOT/config"
HARDWARE_DIR="$CONFIG_DIR/hardware"
MY_HARDWARE_DIR="$CONFIG_DIR/my-hardware"
TARGET_CONFIG="$CONFIG_DIR/config.yaml"

# Farbcodes für bessere Lesbarkeit
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Hardware-spezifische Konfigurationsschlüssel, die beim Wechsel aktualisiert werden sollten
# Hier definieren wir, welche Konfigurationseinstellungen hardwarespezifisch sind
HARDWARE_KEYS=(
  "opencv.use_gpu"
  "opencv.backend"
  "opencv.device_id"
  "opencv.metal_enabled"
  "processor.max_workers"
)

# Hilfefunktion
show_help() {
    echo -e "${BLUE}Double-Take-Go-Reborn Hardware-Konfigurationswechsler (verbessert)${NC}"
    echo
    echo "Verwendung: $0 [hardware-typ] [provider-options]"
    echo
    echo "Hardware-Typen:"
    echo "  nvidia    - NVIDIA GPU-Konfiguration verwenden"
    echo "  amd       - AMD GPU-Konfiguration verwenden"
    echo "  cpu       - CPU-Konfiguration verwenden"
    echo "  apple     - Apple Silicon-Konfiguration verwenden"
    echo
    echo "Provider-Optionen:"
    echo "  --compreface=on|off   - CompreFace aktivieren/deaktivieren"
    echo "  --insightface=on|off  - InsightFace aktivieren/deaktivieren"
    echo "  --provider=compreface|insightface|both - Primären Erkennungsprovider setzen"
    echo
    echo "Optionen:"
    echo "  -h, --help         - Diese Hilfe anzeigen"
    echo "  -l, --list         - Verfügbare Konfigurationen auflisten"
    echo "  -s, --status       - Aktuelle Konfiguration anzeigen"
    echo "  -i, --interactive  - Interaktiven Assistenten starten"
    echo "  --setup            - Ersteinrichtung durchführen (persönliches Hardware-Verzeichnis erstellen)"
    echo
    echo "Beispiele:"
    echo "  $0                  - Startet den interaktiven Assistenten"
    echo "  $0 cpu              - Wechselt zur CPU-Konfiguration"
    echo "  $0 -i               - Startet den interaktiven Assistenten"
    echo "  $0 cpu --compreface=off         - Deaktiviert CompreFace in der CPU-Konfiguration"
    echo "  $0 cpu --insightface=on         - Aktiviert InsightFace in der CPU-Konfiguration"
    echo "  $0 cpu --provider=insightface   - Setzt InsightFace als primären Provider"
    echo
    echo "HINWEIS: Diese verbesserte Version erhält alle bestehenden Einstellungen wie"
    echo "         Home Assistant Integration und Provider-Konfiguration beim Wechsel."
}

# Erstelle my-hardware, falls es nicht existiert
init_config() {
    if [ ! -d "$MY_HARDWARE_DIR" ]; then
        echo -e "${YELLOW}Persönliches Hardware-Verzeichnis nicht gefunden. Erstelle...${NC}"
        mkdir -p "$MY_HARDWARE_DIR"
        
        # Kopiere Basis-Konfigurationen, wenn vorhanden
        if [ -d "$HARDWARE_DIR" ] && [ -n "$(ls -A "$HARDWARE_DIR"/*.yaml 2>/dev/null)" ]; then
            echo -e "${BLUE}Kopiere Basis-Konfigurationen...${NC}"
            cp "$HARDWARE_DIR"/*.yaml "$MY_HARDWARE_DIR"/
            echo -e "${GREEN}Basiskonfigurationen wurden kopiert.${NC}"
            echo -e "${YELLOW}WICHTIG: Bitte passe deine persönlichen Einstellungen in den Dateien unter $MY_HARDWARE_DIR an!${NC}"
        else
            echo -e "${RED}Keine Basiskonfigurationen im hardware-Verzeichnis gefunden.${NC}"
            echo -e "${YELLOW}Bitte stelle sicher, dass hardware-Konfigurationen in $HARDWARE_DIR existieren.${NC}"
        fi
    fi
}

# Verfügbare Konfigurationen auflisten
list_configs() {
    echo -e "${BLUE}Verfügbare Hardware-Konfigurationen:${NC}"
    
    if [ -d "$MY_HARDWARE_DIR" ]; then
        echo -e "${GREEN}Persönliche Konfigurationen:${NC}"
        for file in "$MY_HARDWARE_DIR"/config-*.yaml; do
            if [ -f "$file" ]; then
                name=$(basename "$file" | sed 's/config-\(.*\)\.yaml/\1/')
                echo "  - $name"
            fi
        done
    else
        echo -e "${YELLOW}Keine persönlichen Konfigurationen gefunden.${NC}"
    fi
    
    echo -e "\n${GREEN}Standard-Konfigurationen:${NC}"
    for file in "$HARDWARE_DIR"/config-*.yaml; do
        if [ -f "$file" ]; then
            name=$(basename "$file" | sed 's/config-\(.*\)\.yaml/\1/')
            echo "  - $name"
        fi
    done
}

# Aktuelle Konfiguration anzeigen
show_status() {
    if [ -f "$TARGET_CONFIG" ]; then
        echo -e "${BLUE}Aktuelle Konfiguration:${NC}"
        
        # Versuche, den Konfigurationstyp zu erkennen
        hardware_type="Unbekannt"
        
        # Hardware-Typ aus der Konfiguration extrahieren
        if grep -q "backend: \"cuda\"" "$TARGET_CONFIG"; then
            hardware_type="NVIDIA GPU"
        elif grep -q "backend: \"opencl\"" "$TARGET_CONFIG"; then
            hardware_type="AMD GPU"
        elif grep -q "use_gpu: false" "$TARGET_CONFIG"; then
            hardware_type="CPU"
        elif grep -q "metal" "$TARGET_CONFIG"; then
            hardware_type="Apple Silicon"
        fi
        
        # MQTT-Status überprüfen
        mqtt_enabled=$(grep "mqtt:" -A 2 "$TARGET_CONFIG" | grep "enabled:" | awk '{print $2}')
        mqtt_broker=$(grep "mqtt:" -A 3 "$TARGET_CONFIG" | grep "broker:" | awk '{print $2}')
        
        # CompreFace-Status überprüfen
        cf_enabled=$(grep "compreface:" -A 2 "$TARGET_CONFIG" | grep "enabled:" | awk '{print $2}')
        
        # InsightFace-Status überprüfen
        if_enabled=$(grep "insightface:" -A 2 "$TARGET_CONFIG" | grep "enabled:" | awk '{print $2}')
        
        # Primären Provider überprüfen
        provider=$(grep "face_recognition_provider:" "$TARGET_CONFIG" | awk '{print $2}' | tr -d '"')
        
        echo -e "  Typ: ${GREEN}$hardware_type${NC}"
        echo -e "  MQTT: ${mqtt_enabled:-false} ${mqtt_broker:+(Broker: $mqtt_broker)}"
        echo -e "  CompreFace: ${cf_enabled:-false}"
        echo -e "  InsightFace: ${if_enabled:-false}"
        echo -e "  Primärer Provider: ${provider:-compreface}"
        
        echo
        echo -e "${YELLOW}Hinweis: Diese Analyse basiert auf Mustern in der Konfigurationsdatei und ist möglicherweise nicht 100% akkurat.${NC}"
    else
        echo -e "${RED}Keine aktive Konfiguration gefunden.${NC}"
    fi
}

# Provider-Konfiguration aktualisieren
update_provider_config() {
    local config_file=$1
    local provider_option=$2
    local value=$3
    
    case $provider_option in
        compreface)
            # CompreFace aktivieren/deaktivieren
            local cf_line=$(grep -n "^compreface:" "$config_file" | cut -d":" -f1)
            if [ -n "$cf_line" ]; then
                # Die nächste Zeile mit "enabled:" finden und ersetzen
                local next_line=$((cf_line + 1))
                sed -i.bak "${next_line}s/enabled: .*/enabled: $value/" "$config_file"
                echo -e "${BLUE}CompreFace-Konfiguration aktualisiert${NC}"
            else
                echo -e "${RED}CompreFace-Konfiguration nicht gefunden${NC}"
            fi
            ;;
        insightface)
            # InsightFace aktivieren/deaktivieren
            local if_line=$(grep -n "^insightface:" "$config_file" | cut -d":" -f1)
            if [ -n "$if_line" ]; then
                # Die nächste Zeile mit "enabled:" finden und ersetzen
                local next_line=$((if_line + 1))
                sed -i.bak "${next_line}s/enabled: .*/enabled: $value/" "$config_file"
                echo -e "${BLUE}InsightFace-Konfiguration aktualisiert${NC}"
            else
                echo -e "${RED}InsightFace-Konfiguration nicht gefunden${NC}"
            fi
            ;;
        provider)
            # Primären Provider setzen
            if grep -q "face_recognition_provider:" "$config_file"; then
                sed -i.bak "s/face_recognition_provider: .*/face_recognition_provider: \"$value\"/" "$config_file"
                echo -e "${BLUE}Provider-Konfiguration aktualisiert${NC}"
            else
                # Zeile hinzufügen, falls noch nicht vorhanden (am Ende der Datei)
                echo "" >> "$config_file"
                echo "face_recognition_provider: \"$value\"" >> "$config_file"
                echo -e "${BLUE}Provider-Konfiguration hinzugefügt${NC}"
            fi
            ;;
    esac
    
    # Backup-Datei entfernen
    rm -f "${config_file}.bak"
}

# Hardware-spezifische Konfiguration aktualisieren
update_hardware_config() {
    local target_config=$1
    local source_config=$2
    local temp_file="${target_config}.temp"

    # Sicherheitskopie erstellen
    cp "$target_config" "${target_config}.backup"
    echo -e "${BLUE}Sicherheitskopie der aktuellen Konfiguration erstellt: ${target_config}.backup${NC}"
    
    # Für jeden Hardware-Schlüssel
    for key in "${HARDWARE_KEYS[@]}"; do
        # Konvertiere Punkt-Notation in grep-Muster
        local base_key=$(echo "$key" | cut -d'.' -f1)
        local subkey=$(echo "$key" | cut -d'.' -f2-)
        
        # Extrahiere den Wert aus der Quellkonfiguration
        local source_value=""
        if [[ "$subkey" == *"."* ]]; then
            # Hierarchische Konfiguration mit mehreren Ebenen
            local parts=(${subkey//./ })
            local pattern="$base_key:"
            for part in "${parts[@]}"; do
                pattern="$pattern\|[[:space:]]*$part:"
            done
            source_value=$(grep -A 1 "$pattern" "$source_config" | tail -n 1 | sed 's/^[[:space:]]*//g')
        else
            # Einfache Konfiguration mit nur einer Ebene
            source_value=$(grep -A 1 "$base_key:[[:space:]]*$" "$source_config" | grep "$subkey:" | sed 's/^[[:space:]]*//g')
        fi
        
        if [ -n "$source_value" ]; then
            # Aktualisiere den Wert in der Zielkonfiguration
            if grep -q "$base_key:" "$target_config"; then
                if grep -q "$subkey:" "$target_config"; then
                    # Muster: 'subkey: anyvalue' mit 'subkey: newvalue' ersetzen
                    local subkey_pattern=$(echo "$subkey" | cut -d'.' -f1)
                    sed -i.bak "s/^[[:space:]]*$subkey_pattern:.*/$source_value/" "$target_config"
                fi
            fi
        fi
    done
    
    # Spezialbehandlung für use_gpu
    if grep -q "use_gpu:" "$source_config"; then
        local use_gpu=$(grep "use_gpu:" "$source_config" | awk '{print $2}')
        sed -i.bak "s/use_gpu:.*/use_gpu: $use_gpu/" "$target_config"
    fi
    
    # Spezialbehandlung für backend
    if grep -q "backend:" "$source_config"; then
        local backend=$(grep "backend:" "$source_config" | awk '{print $2}')
        if grep -q "backend:" "$target_config"; then
            sed -i.bak "s/backend:.*/backend: $backend/" "$target_config"
        else
            # Füge es in den opencv-Abschnitt ein, falls noch nicht vorhanden
            local opencv_line=$(grep -n "^opencv:" "$target_config" | cut -d":" -f1)
            if [ -n "$opencv_line" ]; then
                local insert_line=$((opencv_line + 3)) # Nach use_gpu
                sed -i.bak "${insert_line}i\\  backend: $backend" "$target_config"
            fi
        fi
    fi
    
    # Spezialbehandlung für metal_enabled (Apple Silicon)
    if grep -q "metal_enabled:" "$source_config"; then
        local metal_enabled=$(grep "metal_enabled:" "$source_config" | awk '{print $2}')
        if grep -q "metal_enabled:" "$target_config"; then
            sed -i.bak "s/metal_enabled:.*/metal_enabled: $metal_enabled/" "$target_config"
        else
            # Füge es in den opencv-Abschnitt ein, falls noch nicht vorhanden
            local opencv_line=$(grep -n "^opencv:" "$target_config" | cut -d":" -f1)
            if [ -n "$opencv_line" ]; then
                local insert_line=$((opencv_line + 3)) # Nach use_gpu
                sed -i.bak "${insert_line}i\\  metal_enabled: $metal_enabled" "$target_config"
            fi
        fi
    fi
    
    # Spezialbehandlung für max_workers
    if grep -q "max_workers:" "$source_config"; then
        local max_workers=$(grep "max_workers:" "$source_config" | awk '{print $2}')
        if grep -q "max_workers:" "$target_config"; then
            sed -i.bak "s/max_workers:.*/max_workers: $max_workers/" "$target_config"
        fi
    fi
    
    # Entferne Backup-Datei
    rm -f "${target_config}.bak"
    
    echo -e "${GREEN}Hardware-spezifische Konfiguration aktualisiert.${NC}"
}

# Konfiguration wechseln
switch_config() {
    local hardware=$1
    shift # Ersten Parameter entfernen
    
    # Standardwerte für Konfigurationsdateinamen ableiten
    local config_filename="config-${hardware}.yaml"
    
    case $hardware in
        nvidia|nvidia-gpu)
            source_file="$MY_HARDWARE_DIR/config-nvidia-gpu.yaml"
            if [ ! -f "$source_file" ]; then
                source_file="$HARDWARE_DIR/config-nvidia-gpu.yaml"
            fi
            ;;
        amd|amd-gpu)
            source_file="$MY_HARDWARE_DIR/config-amd-gpu.yaml"
            if [ ! -f "$source_file" ]; then
                source_file="$HARDWARE_DIR/config-amd-gpu.yaml"
            fi
            ;;
        cpu)
            source_file="$MY_HARDWARE_DIR/config-cpu.yaml"
            if [ ! -f "$source_file" ]; then
                source_file="$HARDWARE_DIR/config-cpu.yaml"
            fi
            ;;
        apple|apple-silicon)
            source_file="$MY_HARDWARE_DIR/config-apple-silicon.yaml"
            if [ ! -f "$source_file" ]; then
                source_file="$HARDWARE_DIR/config-apple-silicon.yaml"
            fi
            ;;
        *)
            echo -e "${RED}Unbekannter Hardware-Typ: $hardware${NC}"
            show_help
            exit 1
            ;;
    esac
    
    if [ ! -f "$source_file" ]; then
        echo -e "${RED}Konfigurationsdatei nicht gefunden: $source_file${NC}"
        echo "Bitte überprüfe, ob die Hardware-Konfigurationen existieren."
        exit 1
    fi
    
    # Erstelle die Hauptkonfigurationsdatei, falls sie nicht existiert
    if [ ! -f "$TARGET_CONFIG" ]; then
        echo -e "${YELLOW}Keine Hauptkonfigurationsdatei gefunden. Erstelle eine Basiskonfiguration...${NC}"
        cp "$source_file" "$TARGET_CONFIG"
        echo -e "${GREEN}Basiskonfiguration aus $source_file erstellt.${NC}"
    else
        # Aktualisiere nur die hardware-spezifischen Teile der Konfiguration
        update_hardware_config "$TARGET_CONFIG" "$source_file"
    fi
    
    echo -e "${GREEN}Konfiguration gewechselt zu: $hardware${NC}"
    
    # Provider-Optionen verarbeiten
    for arg in "$@"; do
        if [[ $arg == --compreface=* ]]; then
            local cf_value=${arg#*=}
            if [ "$cf_value" = "on" ]; then
                update_provider_config "$TARGET_CONFIG" "compreface" "true"
                echo -e "${GREEN}CompreFace aktiviert${NC}"
            elif [ "$cf_value" = "off" ]; then
                update_provider_config "$TARGET_CONFIG" "compreface" "false"
                echo -e "${YELLOW}CompreFace deaktiviert${NC}"
            fi
        elif [[ $arg == --insightface=* ]]; then
            local if_value=${arg#*=}
            if [ "$if_value" = "on" ]; then
                update_provider_config "$TARGET_CONFIG" "insightface" "true"
                echo -e "${GREEN}InsightFace aktiviert${NC}"
            elif [ "$if_value" = "off" ]; then
                update_provider_config "$TARGET_CONFIG" "insightface" "false"
                echo -e "${YELLOW}InsightFace deaktiviert${NC}"
            fi
        elif [[ $arg == --provider=* ]]; then
            local provider_value=${arg#*=}
            if [ "$provider_value" = "compreface" ] || [ "$provider_value" = "insightface" ] || [ "$provider_value" = "both" ]; then
                update_provider_config "$TARGET_CONFIG" "provider" "$provider_value"
                echo -e "${GREEN}Primärer Erkennungsprovider auf $provider_value gesetzt${NC}"
            fi
        fi
    done
    
    # Info, wie man den Container neu startet und Angebot zum direkten Ausführen
    echo
    
    # Container-Command basierend auf Hardware-Typ festlegen
    local docker_dir=""
    case $hardware in
        nvidia|nvidia-gpu)
            docker_dir="$PROJECT_ROOT/docker/nvidia"
            ;;
        amd|amd-gpu)
            docker_dir="$PROJECT_ROOT/docker/amd"
            ;;
        cpu)
            docker_dir="$PROJECT_ROOT/docker/cpu"
            ;;
        apple|apple-silicon)
            docker_dir="$PROJECT_ROOT/docker/apple-silicon"
            ;;
    esac
    
    local docker_cmd="cd $docker_dir && docker compose down && docker compose up -d --build"
    echo -e "${YELLOW}Container-Neustart-Befehl:${NC}"
    echo -e "${BLUE}$docker_cmd${NC}"
    echo
    
    # Provider-Einstellungen interaktiv konfigurieren
    echo -e "\n${BLUE}Gesichtserkennungsprovider konfigurieren:${NC}"
    
    # CompreFace Einstellungen
    cf_enabled=$(grep -A 2 "^compreface:" "$TARGET_CONFIG" | grep "enabled:" | awk '{print $2}')
    if [ "$cf_enabled" = "true" ]; then
        echo -e "1) CompreFace: ${GREEN}Aktiviert${NC}"
        read -p "   CompreFace deaktivieren? (j/n): " cf_choice
        if [[ "$cf_choice" =~ ^[jJyY]$ ]]; then
            update_provider_config "$TARGET_CONFIG" "compreface" "false"
        fi
    else
        echo -e "1) CompreFace: ${RED}Deaktiviert${NC}"
        read -p "   CompreFace aktivieren? (j/n): " cf_choice
        if [[ "$cf_choice" =~ ^[jJyY]$ ]]; then
            update_provider_config "$TARGET_CONFIG" "compreface" "true"
        fi
    fi
    
    # InsightFace Einstellungen
    if_enabled=$(grep -A 2 "^insightface:" "$TARGET_CONFIG" | grep "enabled:" | awk '{print $2}')
    if [ "$if_enabled" = "true" ]; then
        echo -e "2) InsightFace: ${GREEN}Aktiviert${NC}"
        read -p "   InsightFace deaktivieren? (j/n): " if_choice
        if [[ "$if_choice" =~ ^[jJyY]$ ]]; then
            update_provider_config "$TARGET_CONFIG" "insightface" "false"
        fi
    else
        echo -e "2) InsightFace: ${RED}Deaktiviert${NC}"
        read -p "   InsightFace aktivieren? (j/n): " if_choice
        if [[ "$if_choice" =~ ^[jJyY]$ ]]; then
            update_provider_config "$TARGET_CONFIG" "insightface" "true"
        fi
    fi
    
    # Primärer Provider
    echo -e "\n${BLUE}Primären Erkennungsprovider wählen:${NC}"
    provider=$(grep "face_recognition_provider:" "$TARGET_CONFIG" | awk '{print $2}' | tr -d '"')
    echo -e "   Aktuell: ${GREEN}${provider:-compreface}${NC}"
    echo "   1. CompreFace"
    echo "   2. InsightFace"
    echo "   3. Beide (both)"
    read -p "   Wähle den primären Provider (1-3, Enter für keine Änderung): " provider_choice
    
    case "$provider_choice" in
        1)
            update_provider_config "$TARGET_CONFIG" "provider" "compreface"
            ;;
        2)
            update_provider_config "$TARGET_CONFIG" "provider" "insightface"
            ;;
        3)
            update_provider_config "$TARGET_CONFIG" "provider" "both"
            ;;
    esac
    
    # Fragen, ob der Container neu gestartet werden soll
    echo
    read -p "Möchtest du den Container jetzt neu starten? (j/n): " restart_choice
    
    if [[ "$restart_choice" =~ ^[jJyY]$ ]]; then
        echo -e "${GREEN}Führe Container-Neustart aus...${NC}"
        pushd "$docker_dir" > /dev/null
        docker compose down
        
        # Fragen, ob ein Clean-Build durchgeführt werden soll
        read -p "Clean-Build durchführen (--no-cache)? Empfohlen bei Template/Config-Änderungen (j/n): " clean_build_choice
        
        echo -e "${YELLOW}Container gestoppt. Baue neues Image und starte Container...${NC}"
        if [[ "$clean_build_choice" =~ ^[jJyY]$ ]]; then
            echo -e "${BLUE}Führe Clean-Build aus (--no-cache)...${NC}"
            docker compose build --no-cache
        else
            echo -e "${BLUE}Führe normalen Build aus...${NC}"
            docker compose build
        fi
        
        docker compose up -d
        
        echo -e "${GREEN}Container erfolgreich neu gestartet!${NC}"
        popd > /dev/null
        
        # Optional: Anzeigen der Logs
        read -p "Möchtest du die Container-Logs anzeigen? (j/n): " logs_choice
        if [[ "$logs_choice" =~ ^[jJyY]$ ]]; then
            pushd "$docker_dir" > /dev/null
            echo -e "${BLUE}Container-Logs (Strg+C zum Beenden):${NC}"
            docker compose logs -f
            popd > /dev/null
        fi
    else
        echo -e "${YELLOW}Container-Neustart übersprungen.${NC}"
    fi
}

# Prüfen und ggf. initialisieren von Verzeichnissen, bevor irgendwas anderes passiert
if [ ! -d "$CONFIG_DIR" ]; then
    echo -e "${RED}Fehler: Konfigurationsverzeichnis nicht gefunden: $CONFIG_DIR${NC}"
    echo -e "${YELLOW}Bitte stelle sicher, dass du das Skript aus dem richtigen Verzeichnis aufrufst.${NC}"
    exit 1
fi

# Interaktive Menüauswahl für Hardware und Provider
show_interactive_menu() {
    clear
    echo -e "${BLUE}====== Double-Take-Go-Reborn Konfigurationsassistent ======${NC}"
    echo
    echo -e "${YELLOW}Hardware-Auswahl:${NC}"
    echo "1. CPU-Konfiguration"
    echo "2. NVIDIA GPU-Konfiguration"
    echo "3. AMD GPU-Konfiguration"
    echo "4. Apple Silicon-Konfiguration"
    echo
    echo "q. Beenden"
    echo
    read -p "Bitte wähle eine Option (1-4, q): " menu_choice
    
    local hardware_type=""
    
    case "$menu_choice" in
        1) hardware_type="cpu" ;;
        2) hardware_type="nvidia" ;;
        3) hardware_type="amd" ;;
        4) hardware_type="apple" ;;
        q|Q) echo "Abgebrochen."; exit 0 ;;
        *) echo -e "${RED}Ungültige Auswahl!${NC}"; return 1 ;;
    esac
    
    # Konfiguration wechseln
    if [ -n "$hardware_type" ]; then
        switch_config "$hardware_type"
        return 0
    fi
    
    return 1
}

# Hauptprogramm
case "$1" in
    -h|--help)
        show_help
        ;;
    -l|--list)
        list_configs
        ;;
    -s|--status)
        show_status
        ;;
    -i|--interactive)
        show_interactive_menu
        ;;
    --setup)
        echo -e "${BLUE}Führe Ersteinrichtung durch...${NC}"
        init_config
        echo -e "${GREEN}Ersteinrichtung abgeschlossen.${NC}"
        echo -e "${YELLOW}Verwende '$0 --list' um verfügbare Konfigurationen anzuzeigen.${NC}"
        ;;
    "")
        # Prüfen, ob das Hardware-Verzeichnis existiert, andernfalls Hinweis zum Setup
        if [ ! -d "$MY_HARDWARE_DIR" ] || [ -z "$(ls -A "$MY_HARDWARE_DIR" 2>/dev/null)" ]; then
            echo -e "${YELLOW}Persönliches Hardware-Verzeichnis fehlt oder ist leer.${NC}"
            echo -e "${BLUE}Führe zuerst '$0 --setup' aus, um die Ersteinrichtung durchzuführen.${NC}"
            exit 1
        fi
        # Interaktives Menü ohne Parameter anzeigen
        show_interactive_menu
        ;;
    *)
        # Prüfen, ob das Hardware-Verzeichnis existiert, andernfalls automatisch initialisieren
        if [ ! -d "$MY_HARDWARE_DIR" ]; then
            echo -e "${YELLOW}Persönliches Hardware-Verzeichnis nicht gefunden. Führe automatische Ersteinrichtung durch...${NC}"
            init_config
        fi
        switch_config "$@"
        ;;
esac

exit 0
