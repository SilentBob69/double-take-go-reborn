# Stage 1: Builder
FROM golang:1.24-bullseye AS builder

WORKDIR /app

# OpenCV-Abhängigkeiten installieren
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    pkg-config \
    libopencv-dev \
    libopencv-contrib-dev \
    ca-certificates \
    git \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# Abhängigkeiten kopieren und herunterladen
COPY go.mod go.sum ./
RUN go mod download

# Quellcode kopieren
COPY . .

# Anwendung bauen
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o double-take ./cmd/server

# Stage 2: Runner
FROM debian:bullseye-slim

WORKDIR /app

# Abhängigkeiten installieren
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    mosquitto-clients \
    jq \
    curl \
    libopencv-core4.5 \
    libopencv-imgproc4.5 \
    libopencv-imgcodecs4.5 \
    libopencv-objdetect4.5 \
    libopencv-dnn4.5 \
    && rm -rf /var/lib/apt/lists/*

# Zeitzone-Daten beibehalten
ENV TZ=Europe/Berlin

# Anwendungsbinär vom Builder-Stage kopieren
COPY --from=builder /app/double-take /app/double-take

# Web-Dateien kopieren
COPY --from=builder /app/web /app/web

# Datenverzeichnisse erstellen
RUN mkdir -p /data/snapshots /config

# Volumes definieren
VOLUME ["/data", "/config"]

# Port freigeben
EXPOSE 3000

# Startbefehl
CMD ["/app/double-take"]
