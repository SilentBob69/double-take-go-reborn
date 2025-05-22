#!/bin/bash
# switch-config.sh - Robuste Version des Konfigurationswechslers
# Autor: Double-Take Team
# Datum: 2025-05-21

set -e  # Bei Fehlern abbrechen

# Pfade automatisch ermitteln
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CONFIG_DIR="$PROJECT_ROOT/config"
MY_HARDWARE_DIR="$CONFIG_DIR/my-hardware"  # Hier liegen die Hauptkonfigurationen
HARDWARE_DIR="$CONFIG_DIR/hardware"        # Fallback-Verzeichnis
TARGET_CONFIG="$CONFIG_DIR/config.yaml"

# Farbcodes für bessere Lesbarkeit
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Einen Konfigurationswert in einer YAML-Datei ändern
update_yaml_value() {
    local file=$1
    local key=$2
    local value=$3
    local tmpfile=$(mktemp)
    
    while IFS= read -r line; do
        if [[ $line =~ ^[[:space:]]*$key:[[:space:]] ]]; then
            # Einrückung beibehalten
            indent=$(echo "$line" | sed -E "s/^([[:space:]]*)$key:.*/\\1/")
            echo "${indent}$key: $value"
        else
            echo "$line"
        fi
    done < "$file" > "$tmpfile"
    
    # Ersetze die Originaldatei mit der bearbeiteten Datei
    mv "$tmpfile" "$file"
}

# Hilfefunktion
show_help() {
    echo -e "${BLUE}Double-Take-Go-Reborn Konfigurationswechsler${NC}"
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
    echo "  --provider=compreface|insightface|both - Primären Erkennungsanbieter setzen"
    echo
    echo "Optionen:"
    echo "  -h, --help      - Diese Hilfe anzeigen"
    echo "  -l, --list      - Verfügbare Konfigurationen auflisten"
    echo "  -s, --status    - Aktuelle Konfiguration anzeigen"
    echo "  -i, --interactive - Interaktiven Assistenten starten"
    echo "  -L, --logs      - Container-Logs anzeigen (folgen)"
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

# Container-Logs anzeigen
show_logs() {
    # Hardware-Typ als Parameter übernehmen oder aus der Konfiguration ermitteln
    local hardware_type="$1"
    
    # Wenn kein Hardware-Typ übergeben wurde, aus der Konfiguration ermitteln
    if [ -z "$hardware_type" ]; then
        hardware_type="cpu" # Standard-Fallback
        if [ -f "$TARGET_CONFIG" ]; then
            if grep -q "backend: \"cuda\"" "$TARGET_CONFIG"; then
                hardware_type="nvidia"
            elif grep -q "backend: \"opencl\"" "$TARGET_CONFIG"; then
                hardware_type="amd"
            elif grep -q "metal_enabled: true" "$TARGET_CONFIG"; then
                hardware_type="apple"
            fi
        fi
    fi
    
    echo -e "${BLUE}Hardware-Typ für Logs: $hardware_type${NC}"
    
    # Docker-Verzeichnis bestimmen - konsistent mit der switch_config-Funktion
    local docker_dir=""
    case $hardware_type in
        nvidia)
            docker_dir="$PROJECT_ROOT/docker/nvidia"
            ;;
        amd)
            docker_dir="$PROJECT_ROOT/docker/amd"
            ;;
        cpu)
            docker_dir="$PROJECT_ROOT/docker/cpu"
            ;;
        apple)
            docker_dir="$PROJECT_ROOT/docker/apple-silicon"
            ;;
        *)
            # Fallback auf CPU, falls der Typ nicht erkannt wird
            docker_dir="$PROJECT_ROOT/docker/cpu"
            echo -e "${YELLOW}Hardware-Typ nicht erkannt, verwende CPU-Container.${NC}"
            ;;
    esac

    # Prüfen, ob Container existiert und läuft
    echo -e "${BLUE}Suche nach laufenden Containern in $docker_dir...${NC}"
    pushd "$docker_dir" > /dev/null
    
    container_name=$(docker compose ps --services 2>/dev/null | grep double-take || echo "")
    if [ -z "$container_name" ]; then
        echo -e "${RED}Kein laufender Double-Take-Container gefunden.${NC}"
        echo -e "${YELLOW}Starten Sie zuerst den Container mit:${NC}"
        echo -e "cd $docker_dir && docker compose up -d"
        popd > /dev/null
        return 1
    fi
    
    echo -e "${GREEN}Container gefunden: $container_name${NC}"
    echo -e "${BLUE}Logs werden angezeigt (Ctrl+C zum Beenden):${NC}"
    echo
    
    # Logs anzeigen
    docker compose logs --follow
    
    popd > /dev/null
}

# Aktuelle Konfiguration anzeigen
show_status() {
    if [ -f "$TARGET_CONFIG" ]; then
        echo -e "${BLUE}Aktuelle Konfiguration:${NC}"
        
        # Hardware-Typ aus der Konfiguration extrahieren
        hardware_type="Unbekannt"
        if grep -q "backend: \"cuda\"" "$TARGET_CONFIG"; then
            hardware_type="NVIDIA GPU"
        elif grep -q "backend: \"opencl\"" "$TARGET_CONFIG"; then
            hardware_type="AMD GPU"
        elif grep -q "use_gpu: false" "$TARGET_CONFIG"; then
            hardware_type="CPU"
        elif grep -q "metal" "$TARGET_CONFIG"; then
            hardware_type="Apple Silicon"
        fi
        
        # Status von OpenCV und Personenerkennung
        opencv_enabled=$(grep "enabled" "$TARGET_CONFIG" | head -1 | awk '{print $2}')
        person_method=$(grep -A 3 "person_detection:" "$TARGET_CONFIG" | grep "method:" | awk '{print $2}' | tr -d '"')
        
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
        echo -e "  OpenCV: ${opencv_enabled:-false}"
        echo -e "  Personenerkennung: ${person_method:-hog}"
        echo -e "  MQTT: ${mqtt_enabled:-false} ${mqtt_broker:+(Broker: $mqtt_broker)}"
        echo -e "  CompreFace: ${cf_enabled:-false}"
        echo -e "  InsightFace: ${if_enabled:-false}"
        echo -e "  Primärer Provider: ${provider:-compreface}"
    else
        echo -e "${RED}Keine aktive Konfiguration gefunden.${NC}"
    fi
}

# Konfigurationsanbieter aktualisieren
update_provider_config() {
    local config_file=$1
    local provider_option=$2
    local value=$3
    
    case $provider_option in
        compreface)
            # Suche nach compreface:-Block und ändere den enabled-Wert
            local tmpfile=$(mktemp)
            local in_compreface_block=false
            local changed=false
            
            while IFS= read -r line; do
                if [[ "$line" =~ ^compreface: ]]; then
                    in_compreface_block=true
                    echo "$line"
                elif [[ "$in_compreface_block" == true && "$line" =~ ^[[:space:]]*enabled: ]]; then
                    echo "  enabled: $value"
                    changed=true
                    in_compreface_block=false
                else
                    echo "$line"
                fi
            done < "$config_file" > "$tmpfile"
            
            if [[ "$changed" == true ]]; then
                mv "$tmpfile" "$config_file"
                # Stille Ausführung
            else
                rm "$tmpfile"
                # Stille Ausführung bei Fehlern
            fi
            ;;
        insightface)
            # Suche nach insightface:-Block und ändere den enabled-Wert
            local tmpfile=$(mktemp)
            local in_insightface_block=false
            local changed=false
            local block_exists=false
            
            # Prüfen, ob der Block überhaupt existiert
            if grep -q "^insightface:" "$config_file"; then
                block_exists=true
            fi
            
            if [[ "$block_exists" == true ]]; then
                # Block existiert - nur enabled-Wert ändern
                while IFS= read -r line; do
                    if [[ "$line" =~ ^insightface: ]]; then
                        in_insightface_block=true
                        echo "$line"
                    elif [[ "$in_insightface_block" == true && "$line" =~ ^[[:space:]]*enabled: ]]; then
                        echo "  enabled: $value"
                        changed=true
                        in_insightface_block=false
                    else
                        echo "$line"
                    fi
                done < "$config_file" > "$tmpfile"
                
                if [[ "$changed" == true ]]; then
                    mv "$tmpfile" "$config_file"
                else
                    rm "$tmpfile"
                fi
            else
                # Block existiert nicht - hinzufügen wenn aktiviert werden soll
                if [[ "$value" == "true" ]]; then
                    echo "# InsightFace-Block existiert nicht, aber soll aktiviert werden"
                    echo "# Füge Standard-Block am Ende der Datei hinzu"
                    cat "$config_file" > "$tmpfile"
                    echo "" >> "$tmpfile"
                    echo "# =========================================================================" >> "$tmpfile"
                    echo "# InsightFace-Integration für Gesichtserkennung (Alternative zu CompreFace)" >> "$tmpfile"
                    echo "# =========================================================================" >> "$tmpfile"
                    echo "insightface:" >> "$tmpfile"
                    echo "  enabled: true" >> "$tmpfile"
                    echo "  url: \"http://insightface:18081\"" >> "$tmpfile"
                    echo "  detect_url: \"http://insightface:18081/extract\"" >> "$tmpfile"
                    echo "  recognize_url: \"http://insightface:18081/recognize\"" >> "$tmpfile"
                    echo "  add_face_url: \"http://insightface:18081/add_face\"" >> "$tmpfile"
                    echo "  detection_threshold: 0.6" >> "$tmpfile"
                    echo "  recognition_threshold: 0.5" >> "$tmpfile"
                    echo "  max_faces: 10" >> "$tmpfile"
                    echo "  status_check_interval: 1" >> "$tmpfile"
                    echo "  status_check_timeout: 30" >> "$tmpfile"
                    mv "$tmpfile" "$config_file"
                    # Stille Ausführung ohne Fehlermeldung
                else
                    # Wenn InsightFace deaktiviert werden soll, aber der Block nicht existiert,
                    # müssen wir nichts tun
                    rm "$tmpfile"
                fi
            fi
            ;;
        provider)
            # Primären Provider setzen
            if grep -q "face_recognition_provider:" "$config_file"; then
                # Provider aktualisieren
                local tmpfile=$(mktemp)
                while IFS= read -r line; do
                    if [[ "$line" =~ face_recognition_provider: ]]; then
                        # Stelle sicher, dass der Wert ohne Anführungszeichen ist und dann füge sie korrekt hinzu
                        local clean_value=$(echo "$value" | sed 's/"//g')
                        echo "  face_recognition_provider: \"$clean_value\"" 
                    else
                        echo "$line"
                    fi
                done < "$config_file" > "$tmpfile"
                mv "$tmpfile" "$config_file"
                # Stille Ausführung
            else
                # Prüfen, ob ein processor-Block existiert
                if grep -q "^processor:" "$config_file"; then
                    # Füge face_recognition_provider in den processor-Block ein
                    local tmpfile=$(mktemp)
                    local in_processor_block=false
                    local provider_added=false
                    
                    while IFS= read -r line; do
                        echo "$line"
                        
                        # Wenn wir im processor-Block sind und die letzte Zeile des Blocks erreicht haben
                        if [[ "$in_processor_block" == true && ! "$line" =~ ^[[:space:]] ]] && [[ "$provider_added" == false ]]; then
                            # Füge den Provider vor dem Ende des Blocks ein
                            echo "  # Provider für die Gesichtserkennung: \"compreface\" oder \"insightface\"" >> "$tmpfile"
                            echo "  face_recognition_provider: \"$value\"" >> "$tmpfile"
                            provider_added=true
                            in_processor_block=false
                        elif [[ "$line" =~ ^processor: ]]; then
                            in_processor_block=true
                        elif [[ "$in_processor_block" == true && ! "$line" =~ ^[[:space:]] ]]; then
                            # Ende des processor-Blocks erreicht
                            in_processor_block=false
                        fi
                    done < "$config_file" > "$tmpfile"
                    
                    # Wenn wir am Ende der Datei ankommen und noch im processor-Block sind
                    if [[ "$in_processor_block" == true && "$provider_added" == false ]]; then
                        echo "  # Provider für die Gesichtserkennung: \"compreface\" oder \"insightface\"" >> "$tmpfile"
                        echo "  face_recognition_provider: \"$value\"" >> "$tmpfile"
                    fi
                    
                    mv "$tmpfile" "$config_file"
                    # Stille Ausführung
                else
                    # Wenn kein processor-Block existiert, fügen wir einen hinzu
                    echo "" >> "$config_file"
                    echo "processor:" >> "$config_file"
                    echo "  image_processing_interval: 5" >> "$config_file"
                    echo "  max_workers: 5" >> "$config_file"
                    echo "  max_processing_time: 30" >> "$config_file"
                    echo "  # Provider für die Gesichtserkennung: \"compreface\" oder \"insightface\"" >> "$config_file"
                    echo "  face_recognition_provider: \"$value\"" >> "$config_file"
                    # Stille Ausführung
                fi
            fi
            ;;
    esac
}

# Hardware-spezifische Felder aus der Quellkonfiguration extrahieren
extract_hardware_config() {
    local source_config=$1
    local target_config=$2
    
    # 1. use_gpu-Wert übernehmen
    if grep -q "use_gpu:" "$source_config"; then
        local use_gpu=$(grep "use_gpu:" "$source_config" | awk '{print $2}')
        update_yaml_value "$target_config" "use_gpu" "$use_gpu"
    fi
    
    # 2. backend-Wert übernehmen
    if grep -q "backend:" "$source_config"; then
        # Extrahiere den Wert ohne Anführungszeichen
        local backend=$(grep "backend:" "$source_config" | awk '{print $2}' | sed 's/"//g')
        if grep -q "backend:" "$target_config"; then
            update_yaml_value "$target_config" "backend" "$backend"
        else
            # Backend in backend-Block einfügen, falls nicht vorhanden
            if grep -q "opencv:" "$TARGET_CONFIG"; then
                # Nach dem opencv-Block suchen und backend dort einfügen
                local tmp=$(mktemp)
                # Anführungszeichen um backend-Wert setzen, damit YAML korrekt ist
                awk -v backend="$backend" '
                /opencv:/ { print; in_block = 1; next }
                in_block && /enabled:/ { print "    enabled: true\n    backend: \"" backend "\""; in_block = 0; next }
                { print }
                ' "$TARGET_CONFIG" > "$tmp" && mv "$tmp" "$TARGET_CONFIG"
                rm -f "${TARGET_CONFIG}.bak" 2>/dev/null
            fi
        fi
    fi
    
    # 3. target-Wert übernehmen
    if grep -q "target:" "$source_config"; then
        # Extrahiere den Wert ohne Anführungszeichen
        local target=$(grep "target:" "$source_config" | awk '{print $2}' | sed 's/"//g')
        if grep -q "target:" "$target_config"; then
            update_yaml_value "$target_config" "target" "$target"
        else
            # Target in den opencv-Block einfügen, falls nicht vorhanden
            if grep -q "opencv:" "$TARGET_CONFIG"; then
                # Nach dem backend-Eintrag suchen und target dort einfügen
                local tmp=$(mktemp)
                # Anführungszeichen um target-Wert setzen, damit YAML korrekt ist
                awk -v target="$target" '
                /backend:/ { print; print "  target: \"" target "\""; next }
                { print }
                ' "$TARGET_CONFIG" > "$tmp" && mv "$tmp" "$TARGET_CONFIG"
                rm -f "${TARGET_CONFIG}.bak" 2>/dev/null
            fi
        fi
    fi
    
    # 3. metal_enabled-Wert übernehmen (für Apple Silicon)
    if grep -q "metal_enabled:" "$source_config"; then
        local metal_enabled=$(grep "metal_enabled:" "$source_config" | awk '{print $2}')
        if grep -q "metal_enabled:" "$target_config"; then
            update_yaml_value "$target_config" "metal_enabled" "$metal_enabled"
        else
            # Metal-Wert einfügen, falls benötigt
            if grep -q "opencv:" "$TARGET_CONFIG"; then
                # Nach dem opencv-Block suchen und metal_enabled dort einfügen
                # Plattformunabhängiger Ansatz ohne komplexe sed-Befehle
                tmp=$(mktemp)
                awk -v metal_enabled="$metal_enabled" '
                /opencv:/ { print; in_block = 1; next }
                in_block && /backend:/ { print "    backend: \"metal\"\n    metal_enabled: " metal_enabled; in_block = 0; next }
                { print }
                ' "$TARGET_CONFIG" > "$tmp" && mv "$tmp" "$TARGET_CONFIG"
                rm -f "${TARGET_CONFIG}.bak" 2>/dev/null
            fi
        fi
    fi
    
    # 4. max_workers-Wert übernehmen
    if grep -q "max_workers:" "$source_config"; then
        local max_workers=$(grep "max_workers:" "$source_config" | awk '{print $2}')
        if grep -q "max_workers:" "$target_config"; then
            update_yaml_value "$target_config" "max_workers" "$max_workers"
        fi
    fi
    
    # 5. Personendetektionsmethode übernehmen
    if grep -q "person_detection:" -A 5 "$source_config"; then
        # Extrahiere den Wert ohne Anführungszeichen
        local method=$(grep -A 5 "person_detection:" "$source_config" | grep "method:" | awk '{print $2}' | tr -d '"')
        
        if [ -n "$method" ]; then
            local tmpfile=$(mktemp)
            local in_opencv=false
            local in_person_detection=false
            local method_updated=false
            
            while IFS= read -r line; do
                if [[ "$line" =~ ^[[:space:]]*opencv: ]]; then
                    in_opencv=true
                    echo "$line"
                elif [[ "$in_opencv" == true && "$line" =~ ^[[:space:]]*person_detection: ]]; then
                    in_person_detection=true
                    echo "$line"
                elif [[ "$in_person_detection" == true && "$line" =~ ^[[:space:]]*method: ]]; then
                    # Einrückung beibehalten
                    indent=$(echo "$line" | sed -E "s/^([[:space:]]*)method:.*/\1/")
                    echo "${indent}method: \"$method\"  # Options: hog, dnn"
                    method_updated=true
                else
                    echo "$line"
                fi
                
                # Ende des person_detection-Blocks erkennen
                if [[ "$in_person_detection" == true && "$line" =~ ^[[:space:]]{2}[a-z] && ! "$line" =~ ^[[:space:]]{4} ]]; then
                    in_person_detection=false
                fi
                
                # Ende des OpenCV-Blocks erkennen
                if [[ "$in_opencv" == true && "$line" =~ ^[a-z] ]]; then
                    in_opencv=false
                    in_person_detection=false
                fi
            done < "$target_config" > "$tmpfile"
            
            if [[ "$method_updated" == true ]]; then
                mv "$tmpfile" "$target_config"
                # Stille Ausführung ohne unnötige Bestätigung
            else
                rm "$tmpfile"
                # Stille Ausführung ohne Fehlermeldung
                # Methode wird bei nächster Ausführung aktualisiert
            fi
        fi
    fi
}

# Konfiguration wechseln
switch_config() {
    local hardware=$1
    shift # Ersten Parameter entfernen
    
    # Hardware-Typ definieren
    case $hardware in
        nvidia|nvidia-gpu) 
            config_name="config-nvidia-gpu.yaml" ;;
        amd|amd-gpu) 
            config_name="config-amd-gpu.yaml" ;;
        cpu) 
            config_name="config-cpu.yaml" ;;
        apple|apple-silicon) 
            config_name="config-apple-silicon.yaml" ;;
        *)
            echo -e "${RED}Unbekannter Hardware-Typ: $hardware${NC}"
            show_help
            exit 1
            ;;
    esac
    
    # Richtigen Pfad zur Konfigurationsdatei finden
    source_file=""
    
    # Erst in my-hardware suchen (bevorzugt)
    if [ -f "$MY_HARDWARE_DIR/$config_name" ]; then
        source_file="$MY_HARDWARE_DIR/$config_name"
        echo -e "${BLUE}Verwende benutzerdefinierte Konfiguration: $config_name${NC}"
    # Dann im Standard-Hardware-Verzeichnis suchen
    elif [ -f "$HARDWARE_DIR/$config_name" ]; then
        source_file="$HARDWARE_DIR/$config_name"
        echo -e "${BLUE}Verwende Standard-Konfiguration: $config_name${NC}"
    # Wenn nichts gefunden, im Projekt-Stammverzeichnis suchen
    elif [ -f "$CONFIG_DIR/$config_name" ]; then
        source_file="$CONFIG_DIR/$config_name"
        echo -e "${BLUE}Verwende Konfiguration aus Hauptverzeichnis: $config_name${NC}"
    fi
    
    if [ ! -f "$source_file" ]; then
        echo -e "${RED}Konfigurationsdatei nicht gefunden: $source_file${NC}"
        echo "Bitte überprüfe, ob die Hardware-Konfigurationen existieren."
        exit 1
    fi
    
    # Konfiguration prüfen
    if [ -f "$TARGET_CONFIG" ]; then
        # Konfiguration existiert bereits - weitermachen ohne Backup
        :  # Null-Befehl (no-op)
    else
        # Wenn keine Konfigurationsdatei existiert, die Hardware-Konfiguration komplett übernehmen
        cp "$source_file" "$TARGET_CONFIG"
        echo -e "${GREEN}Neue Konfiguration erstellt aus: $hardware${NC}"
        return 0
    fi
    
    # Hardware-spezifische Felder aus der Quellkonfiguration extrahieren und in die Zielkonfiguration übernehmen
    extract_hardware_config "$source_file" "$TARGET_CONFIG"
    echo -e "${GREEN}Hardware-Konfiguration gewechselt zu: $hardware${NC}"
    
    # Provider-Optionen verarbeiten
    for arg in "$@"; do
        if [[ $arg == --compreface=* ]]; then
            local cf_value=${arg#*=}
            if [ "$cf_value" = "on" ]; then
                update_provider_config "$TARGET_CONFIG" "compreface" "true"
                # Nur eine zusammenfassende Nachricht am Ende
            elif [ "$cf_value" = "off" ]; then
                update_provider_config "$TARGET_CONFIG" "compreface" "false"
                # Nur eine zusammenfassende Nachricht am Ende
            fi
        elif [[ $arg == --insightface=* ]]; then
            local if_value=${arg#*=}
            if [ "$if_value" = "on" ]; then
                update_provider_config "$TARGET_CONFIG" "insightface" "true"
                # Nur eine zusammenfassende Nachricht am Ende
            elif [ "$if_value" = "off" ]; then
                update_provider_config "$TARGET_CONFIG" "insightface" "false"
                # Nur eine zusammenfassende Nachricht am Ende
            fi
        elif [[ $arg == --provider=* ]]; then
            local provider_value=${arg#*=}
            if [ "$provider_value" = "compreface" ] || [ "$provider_value" = "insightface" ] || [ "$provider_value" = "both" ]; then
                update_provider_config "$TARGET_CONFIG" "provider" "$provider_value"
                # Nur eine zusammenfassende Nachricht am Ende
            fi
        fi
    done
    
    # Container-Neustart-Info
    echo
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
    
    # Zusammenfassung der Änderungen anzeigen
    echo -e "\n${GREEN}Folgende Änderungen wurden vorgenommen:${NC}"
    echo -e "- Hardware: ${BLUE}$hardware${NC}"
    
    # CompreFace Status anzeigen
    if [[ "$@" == *"--compreface=on"* ]]; then
        echo -e "- CompreFace: ${GREEN}aktiviert${NC}"
    elif [[ "$@" == *"--compreface=off"* ]]; then
        echo -e "- CompreFace: ${YELLOW}deaktiviert${NC}"
    fi
    
    # InsightFace Status anzeigen
    if [[ "$@" == *"--insightface=on"* ]]; then
        echo -e "- InsightFace: ${GREEN}aktiviert${NC}"
    elif [[ "$@" == *"--insightface=off"* ]]; then
        echo -e "- InsightFace: ${YELLOW}deaktiviert${NC}"
    fi
    
    # Primären Provider anzeigen
    for arg in "$@"; do
        if [[ "$arg" == --provider=* ]]; then
            provider_value=${arg#*=}
            echo -e "- Provider: ${BLUE}$provider_value${NC}"
        fi
    done
    
    # Docker-Restart Optionen anzeigen
    echo -e "\n${YELLOW}Zum Anwenden der Änderungen:${NC}"
    echo -e "cd $docker_dir && docker compose down && docker compose build && docker compose up -d"
    
    # Fragen, ob der Container neu gestartet werden soll
    echo
    read -p "Container jetzt neu starten? (j/n): " restart_choice
    
    if [[ "$restart_choice" =~ ^[jJyY]$ ]]; then
        pushd "$docker_dir" > /dev/null
        echo -e "${BLUE}Stoppe Container...${NC}"
        docker compose down
        
        # Fragen, ob ein Clean-Build durchgeführt werden soll
        read -p "Clean-Build durchführen? (j/n): " clean_build_choice
        
        if [[ "$clean_build_choice" =~ ^[jJyY]$ ]]; then
            echo -e "${BLUE}Führe Clean-Build aus...${NC}"
            docker compose build --no-cache
        else
            echo -e "${BLUE}Führe normalen Build aus...${NC}"
            docker compose build
        fi
        
        echo -e "${BLUE}Starte Container...${NC}"
        docker compose up -d
        
        echo -e "${GREEN}Fertig! Container neu gestartet.${NC}"
        popd > /dev/null
        
        # Fragen, ob Logs angezeigt werden sollen
        echo
        read -p "Möchten Sie die Container-Logs anzeigen? (j/n): " show_logs_choice
        if [[ "$show_logs_choice" =~ ^[jJyY]$ ]]; then
            # Hardware-Typ als Parameter übergeben
            show_logs "$hardware"
        else
            echo -e "${BLUE}Sie können die Logs jederzeit mit diesem Befehl anzeigen:${NC}"
            echo -e "$0 --logs"
            echo -e "oder kürzer:${NC}"
            echo -e "$0 -L"
        fi
    else
        echo -e "${YELLOW}Container-Neustart übersprungen.${NC}"
    fi
}

# Interaktive Menüauswahl für Hardware
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
    
    # Provider-Optionen abfragen
    local provider_args=""
    echo
    echo -e "${YELLOW}Gesichtserkennungs-Provider:${NC}"
    
    # CompreFace aktivieren/deaktivieren
    echo -e "\nCompreFace aktivieren?"
    select cf_choice in "Ja" "Nein" "Nicht ändern"; do
        case $cf_choice in
            "Ja") 
                provider_args="$provider_args --compreface=on"
                break
                ;;
            "Nein")
                provider_args="$provider_args --compreface=off"
                break
                ;;
            "Nicht ändern")
                break
                ;;
            *) echo "Ungültige Auswahl" ;;
        esac
    done
    
    # InsightFace aktivieren/deaktivieren
    echo -e "\nInsightFace aktivieren?"
    select if_choice in "Ja" "Nein" "Nicht ändern"; do
        case $if_choice in
            "Ja") 
                provider_args="$provider_args --insightface=on"
                break
                ;;
            "Nein")
                provider_args="$provider_args --insightface=off"
                break
                ;;
            "Nicht ändern")
                break
                ;;
            *) echo "Ungültige Auswahl" ;;
        esac
    done
    
    # Primären Provider wählen - vereinfacht ohne komplexe Parameter
    echo -e "\nPrimären Erkennungsanbieter wählen:"
    select provider_choice in "CompreFace" "InsightFace" "Beide" "Nicht ändern"; do
        case $provider_choice in
            "CompreFace") 
                # Direkt Variable setzen statt parameter-string
                provider="compreface"
                break
                ;;
            "InsightFace")
                provider="insightface"
                break
                ;;
            "Beide")
                provider="both"
                break
                ;;
            "Nicht ändern")
                provider=""
                break
                ;;
            *) echo "Ungültige Auswahl" ;;
        esac
    done
    
    # Wenn Provider gewählt wurde, als Parameter hinzufügen
    if [ -n "$provider" ]; then
        provider_args="$provider_args --provider=$provider"
    fi
    
    # Konfiguration wechseln
    if [ -n "$hardware_type" ]; then
        # Provider-Argumente an das Kommando anhängen, wenn vorhanden
        if [ -n "$provider_args" ]; then
            switch_config "$hardware_type" $provider_args
        else
            switch_config "$hardware_type"
        fi
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
    -L|--logs)
        show_logs
        ;;
    -i|--interactive)
        show_interactive_menu
        ;;
    "")
        # Interaktives Menü ohne Parameter anzeigen
        show_interactive_menu
        ;;
    *)
        switch_config "$@"
        ;;
esac

exit 0
